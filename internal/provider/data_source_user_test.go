package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
)

func TestUserDataSourceModelFill(t *testing.T) {
	createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	slackKey := "U0123ABCD"

	tests := []struct {
		name         string
		fields       userFields
		wantSlackKey types.String
	}{
		{
			name: "linked to Slack",
			fields: userFields{
				sub: "auth0|abc", nickname: "alice", picture: "https://example.com/a.png", status: "active",
				slackUserKey: &slackKey, phoneVerified: true, createdAt: createdAt,
			},
			wantSlackKey: types.StringValue(slackKey),
		},
		{
			name: "not linked to Slack",
			fields: userFields{
				sub: "auth0|abc", nickname: "alice", picture: "https://example.com/a.png", status: "active",
				createdAt: createdAt,
			},
			wantSlackKey: types.StringNull(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got UserDataSourceModel
			got.fill(tt.fields)

			want := UserDataSourceModel{
				Id:            types.StringValue("auth0|abc"),
				Sub:           types.StringValue("auth0|abc"),
				Nickname:      types.StringValue("alice"),
				SlackUserKey:  tt.wantSlackKey,
				Picture:       types.StringValue("https://example.com/a.png"),
				Status:        types.StringValue("active"),
				PhoneVerified: types.BoolValue(tt.fields.phoneVerified),
				CreatedAt:     types.StringValue("2025-01-02T03:04:05Z"),
			}
			if got != want {
				t.Errorf("fill() = %+v, want %+v", got, want)
			}
		})
	}
}

// testAccUserDataSourceExistingUser returns a user of the test organization. Users cannot be created
// through the API, so the acceptance test reads an existing one. It prefers a user linked to Slack so
// that the lookup by slack_user_key can be tested too.
func testAccUserDataSourceExistingUser(t *testing.T) waroom.User {
	t.Helper()

	client, err := waroom.NewClient(context.Background(),
		waroom.WithBaseURL(os.Getenv(utils.WaroomAPIEndPointEnvName)),
		waroom.WithAPIKey(os.Getenv(utils.WaroomAPIKeyEnvName)),
	)
	if err != nil {
		t.Fatalf("creating Waroom client: %s", err)
	}

	users, err := client.Users.List(context.Background(), waroom.ListUsersParams{})
	if err != nil {
		t.Fatalf("listing users: %s", err)
	}
	if len(users) == 0 {
		t.Skip("the test organization has no users")
	}

	for _, u := range users {
		if !u.SlackUser.Null {
			return u
		}
	}
	return users[0]
}

func TestAccUserDataSource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	user := testAccUserDataSourceExistingUser(t)

	var slackUserKey knownvalue.Check = knownvalue.Null()
	if !user.SlackUser.Null {
		slackUserKey = knownvalue.StringExact(user.SlackUser.Value.Key)
	}

	checks := func(address string) []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(address, tfjsonpath.New("id"), knownvalue.StringExact(user.Sub)),
			statecheck.ExpectKnownValue(address, tfjsonpath.New("sub"), knownvalue.StringExact(user.Sub)),
			statecheck.ExpectKnownValue(address, tfjsonpath.New("nickname"), knownvalue.StringExact(user.Nickname)),
			statecheck.ExpectKnownValue(address, tfjsonpath.New("slack_user_key"), slackUserKey),
			statecheck.ExpectKnownValue(address, tfjsonpath.New("picture"), knownvalue.StringExact(user.Picture)),
			statecheck.ExpectKnownValue(address, tfjsonpath.New("status"), knownvalue.StringExact(string(user.Status))),
			statecheck.ExpectKnownValue(address, tfjsonpath.New("phone_verified"), knownvalue.Bool(user.PhoneVerified)),
			statecheck.ExpectKnownValue(address, tfjsonpath.New("created_at"), knownvalue.StringExact(user.CreatedAt.Format(time.RFC3339))),
		}
	}

	// The error steps come first: the post-test destroy runs with the last step's config, which must be valid.
	steps := []resource.TestStep{
		{
			Config: fmt.Sprintf(`
data "waroom_user" "test" {
  sub      = %[1]q
  nickname = %[2]q
}
`, user.Sub, user.Nickname),
			ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
		},
		{
			Config:      testAccUserDataSourceConfig("sub", acctest.RandomWithPrefix("tf-acc-missing")),
			ExpectError: regexp.MustCompile(`User Not Found`),
		},
		{
			Config:      testAccUserDataSourceConfig("nickname", acctest.RandomWithPrefix("tf-acc-missing")),
			ExpectError: regexp.MustCompile(`User Not Found`),
		},
		{
			Config:            testAccUserDataSourceConfig("sub", user.Sub),
			ConfigStateChecks: checks("data.waroom_user.test"),
		},
		{
			Config:            testAccUserDataSourceConfig("nickname", user.Nickname),
			ConfigStateChecks: checks("data.waroom_user.test"),
		},
	}
	if !user.SlackUser.Null {
		steps = append(steps, resource.TestStep{
			Config:            testAccUserDataSourceConfig("slack_user_key", user.SlackUser.Value.Key),
			ConfigStateChecks: checks("data.waroom_user.test"),
		})
	} else {
		t.Log("no user linked to Slack; skipping the lookup by slack_user_key")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

func testAccUserDataSourceConfig(attr, value string) string {
	return fmt.Sprintf(`
data "waroom_user" "test" {
  %[1]s = %[2]q
}
`, attr, value)
}
