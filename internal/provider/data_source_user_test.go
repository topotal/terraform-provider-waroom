package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"
)

func TestUserDataSourceModelFill(t *testing.T) {
	createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name         string
		slackUserKey *string
		wantSlackKey types.String
	}{
		{name: "linked to Slack", slackUserKey: slackUserKey(false, "U0123ABCD"), wantSlackKey: types.StringValue("U0123ABCD")},
		{name: "not linked to Slack", slackUserKey: slackUserKey(true, ""), wantSlackKey: types.StringNull()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got UserDataSourceModel
			got.fill("auth0|abc", "alice", "https://example.com/a.png", "active", tt.slackUserKey, true, createdAt)

			want := UserDataSourceModel{
				Id:            types.StringValue("auth0|abc"),
				Sub:           types.StringValue("auth0|abc"),
				Nickname:      types.StringValue("alice"),
				SlackUserKey:  tt.wantSlackKey,
				Picture:       types.StringValue("https://example.com/a.png"),
				Status:        types.StringValue("active"),
				PhoneVerified: types.BoolValue(true),
				CreatedAt:     types.StringValue("2025-01-02T03:04:05Z"),
			}
			if got != want {
				t.Errorf("fill() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestMatchUsers(t *testing.T) {
	newUser := func(sub, nickname, slackKey string) waroom.User {
		var u waroom.User
		u.Sub = sub
		u.Nickname = nickname
		u.SlackUser.Null = slackKey == ""
		u.SlackUser.Value.Key = slackKey
		return u
	}
	users := []waroom.User{
		newUser("alice", "alice", "U0001"),
		newUser("alice-upper", "Alice", ""),
		newUser("bob", "bob", "U0002"),
	}

	tests := []struct {
		name     string
		nickname types.String
		slackKey types.String
		wantSubs []string
	}{
		{name: "nickname matches exactly", nickname: types.StringValue("alice"), slackKey: types.StringNull(), wantSubs: []string{"alice"}},
		{name: "nickname is case-sensitive", nickname: types.StringValue("ALICE"), slackKey: types.StringNull(), wantSubs: nil},
		{name: "slack key matches", nickname: types.StringNull(), slackKey: types.StringValue("U0002"), wantSubs: []string{"bob"}},
		{name: "slack key does not match unlinked users", nickname: types.StringNull(), slackKey: types.StringValue(""), wantSubs: nil},
		{name: "no match", nickname: types.StringValue("carol"), slackKey: types.StringNull(), wantSubs: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotSubs []string
			for _, u := range matchUsers(users, tt.nickname, tt.slackKey) {
				gotSubs = append(gotSubs, u.Sub)
			}
			if !slices.Equal(gotSubs, tt.wantSubs) {
				t.Errorf("matchUsers() subs = %v, want %v", gotSubs, tt.wantSubs)
			}
		})
	}
}

// testAccUserDataSourceExistingUser returns an active user of the test organization. Users cannot be
// created through the API, so the acceptance test reads an existing one. Users in other statuses
// (e.g. invitees) may have no nickname yet or change while the test runs, so only active users are
// used. It prefers a user linked to Slack so that the lookup by slack_user_key can be tested too.
func testAccUserDataSourceExistingUser(t *testing.T) waroom.User {
	t.Helper()

	client := testAccWaroomClient(t)
	users, err := client.Users.List(context.Background(), waroom.ListUsersParams{})
	if err != nil {
		t.Fatalf("listing users: %s", err)
	}

	var active []waroom.User
	for _, u := range users {
		if u.Status == "active" && u.Nickname != "" {
			active = append(active, u)
		}
	}
	if len(active) == 0 {
		t.Skip("the test organization has no active users")
	}

	for _, u := range active {
		if !u.SlackUser.Null {
			return u
		}
	}
	return active[0]
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
		// Runs even when no user is linked to Slack: an ignored slack_user_key filter would return
		// other users, which the data source must not mistake for a match.
		{
			Config:      testAccUserDataSourceConfig("slack_user_key", "U"+acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)),
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
