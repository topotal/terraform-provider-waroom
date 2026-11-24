package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
)

func TestAccOnCallTeamDataSource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	const color = "1E90FF"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			// There is no waroom_on_call_team resource yet, so create the team with the client.
			testAccCreateOnCallTeam(t, name, color)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOnCallTeamDataSourceConfig(name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.waroom_on_call_team.by_name", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("data.waroom_on_call_team.by_name", tfjsonpath.New("color"), knownvalue.StringExact(color)),
					statecheck.ExpectKnownValue("data.waroom_on_call_team.by_name", tfjsonpath.New("uuid"), knownvalue.NotNull()),
					statecheck.CompareValuePairs(
						"data.waroom_on_call_team.by_name", tfjsonpath.New("id"),
						"data.waroom_on_call_team.by_name", tfjsonpath.New("uuid"),
						compare.ValuesSame(),
					),
					statecheck.ExpectKnownValue("data.waroom_on_call_team.by_uuid", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("data.waroom_on_call_team.by_uuid", tfjsonpath.New("color"), knownvalue.StringExact(color)),
					statecheck.CompareValuePairs(
						"data.waroom_on_call_team.by_uuid", tfjsonpath.New("id"),
						"data.waroom_on_call_team.by_name", tfjsonpath.New("uuid"),
						compare.ValuesSame(),
					),
				},
			},
			{
				Config:      testAccOnCallTeamDataSourceByNameConfig(acctest.RandomWithPrefix("tf-acc-missing")),
				ExpectError: regexp.MustCompile(`On-Call Team Not Found`),
			},
			{
				Config:      testAccOnCallTeamDataSourceByUUIDConfig("00000000-0000-4000-8000-000000000000"),
				ExpectError: regexp.MustCompile(`On-Call Team Not Found`),
			},
		},
	})
}

// testAccCreateOnCallTeam creates an on-call team with one rotation and no participants,
// and deletes it when the test finishes.
func testAccCreateOnCallTeam(t *testing.T, name, color string) {
	t.Helper()
	ctx := context.Background()

	client, err := waroom.NewClient(ctx,
		waroom.WithBaseURL(os.Getenv(utils.WaroomAPIEndPointEnvName)),
		waroom.WithAPIKey(os.Getenv(utils.WaroomAPIKeyEnvName)),
	)
	if err != nil {
		t.Fatalf("creating Waroom client: %s", err)
	}

	team, err := client.OnCallTeams.Create(ctx, waroom.CreateOnCallTeamReq{
		Name:  name,
		Color: color,
		Rotations: []waroom.CreateOnCallTeamRotationReq{
			{
				Timezone:         "Asia/Tokyo",
				StartedAt:        "2026-01-05T10:00:00+09:00",
				ShiftHandoffTime: "10:00",
				ShiftHandoffWday: waroom.NewOptNillInt(1),
				IntervalDays:     waroom.NewOptInt(7),
			},
		},
	})
	if err != nil {
		t.Fatalf("creating on-call team %q: %s", name, err)
	}

	t.Cleanup(func() {
		err := client.OnCallTeams.Delete(context.Background(), waroom.DeleteOnCallTeamParams{UUID: team.UUID})
		var apiErr *waroom.APIError
		if err != nil && (!errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound) {
			t.Errorf("deleting on-call team %q (%s): %s", name, team.UUID, err)
		}
	})
}

func testAccOnCallTeamDataSourceConfig(name string) string {
	return fmt.Sprintf(`
data "waroom_on_call_team" "by_name" {
  name = %[1]q
}

data "waroom_on_call_team" "by_uuid" {
  uuid = data.waroom_on_call_team.by_name.uuid
}
`, name)
}

func testAccOnCallTeamDataSourceByNameConfig(name string) string {
	return fmt.Sprintf(`
data "waroom_on_call_team" "test" {
  name = %[1]q
}
`, name)
}

func testAccOnCallTeamDataSourceByUUIDConfig(uuid string) string {
	return fmt.Sprintf(`
data "waroom_on_call_team" "test" {
  uuid = %[1]q
}
`, uuid)
}
