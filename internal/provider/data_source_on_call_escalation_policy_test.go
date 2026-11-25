package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	waroom "github.com/topotal/waroom-client-go/waroom"
)

func TestNewOnCallEscalationPolicyDataSourceModel(t *testing.T) {
	var policy waroom.OnCallEscalationPolicy
	if err := policy.UnmarshalJSON([]byte(`{
  "uuid": "770e8400-e29b-41d4-a716-446655440001",
  "name": "Primary escalation",
  "timezone": "Asia/Tokyo",
  "call_depth": 2,
  "call_traversal": "breadth_first",
  "steps": [
    {
      "uuid": "770e8400-e29b-41d4-a716-446655440011",
      "position": 0,
      "step_type": "fallback",
      "simultaneous_call": true,
      "team": {"uuid": "880e8400-e29b-41d4-a716-446655440001", "name": "SRE", "color": "FF0000"}
    }
  ],
  "time_slots": [
    {"uuid": "770e8400-e29b-41d4-a716-446655440021", "day_of_week": 1, "start_time": "22:00", "end_time": "02:00"}
  ]
}`)); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := newOnCallEscalationPolicyDataSourceModel(&policy)

	want := OnCallEscalationPolicyDataSourceModel{
		Id:            types.StringValue("770e8400-e29b-41d4-a716-446655440001"),
		UUID:          types.StringValue("770e8400-e29b-41d4-a716-446655440001"),
		Name:          types.StringValue("Primary escalation"),
		Timezone:      types.StringValue("Asia/Tokyo"),
		CallDepth:     types.Int64Value(2),
		CallTraversal: types.StringValue("breadth_first"),
		Steps: []OnCallEscalationPolicyStepModel{{
			UUID:             types.StringValue("770e8400-e29b-41d4-a716-446655440011"),
			Position:         types.Int64Value(0),
			StepType:         types.StringValue("fallback"),
			SimultaneousCall: types.BoolValue(true),
			Team: OnCallEscalationPolicyStepTeamModel{
				UUID:  types.StringValue("880e8400-e29b-41d4-a716-446655440001"),
				Name:  types.StringValue("SRE"),
				Color: types.StringValue("FF0000"),
			},
		}},
		TimeSlots: []OnCallEscalationPolicyTimeSlotModel{{
			UUID:      types.StringValue("770e8400-e29b-41d4-a716-446655440021"),
			DayOfWeek: types.Int64Value(1),
			StartTime: types.StringValue("22:00"),
			EndTime:   types.StringValue("02:00"),
		}},
	}
	if fmt.Sprintf("%#v", got) != fmt.Sprintf("%#v", want) {
		t.Errorf("model mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNewOnCallEscalationPolicyDataSourceModel_Empty(t *testing.T) {
	var policy waroom.OnCallEscalationPolicy
	if err := policy.UnmarshalJSON([]byte(`{
  "uuid": "770e8400-e29b-41d4-a716-446655440002",
  "name": "Night shift",
  "timezone": "UTC",
  "call_depth": null,
  "call_traversal": "depth_first",
  "steps": [],
  "time_slots": []
}`)); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := newOnCallEscalationPolicyDataSourceModel(&policy)

	if !got.CallDepth.IsNull() {
		t.Errorf("call_depth: want null, got %s", got.CallDepth)
	}
	// Empty (not nil) slices so that the state holds [] rather than null.
	if got.Steps == nil || len(got.Steps) != 0 {
		t.Errorf("steps: want empty non-nil slice, got %#v", got.Steps)
	}
	if got.TimeSlots == nil || len(got.TimeSlots) != 0 {
		t.Errorf("time_slots: want empty non-nil slice, got %#v", got.TimeSlots)
	}
}

func TestAccOnCallEscalationPolicyDataSource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	teamName := acctest.RandomWithPrefix("tf-acc")
	policyName := acctest.RandomWithPrefix("tf-acc")

	const policy = "waroom_on_call_escalation_policy.test"
	const team = "waroom_on_call_team.test"
	const byUUID = "data.waroom_on_call_escalation_policy.by_uuid"
	const byName = "data.waroom_on_call_escalation_policy.by_name"
	stepTeamUUID := func(i int) tfjsonpath.Path {
		return tfjsonpath.New("steps").AtSliceIndex(i).AtMapKey("team").AtMapKey("uuid")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Error cases first: the post-test destroy runs with the last step's config, which must be valid.
			{
				Config:      testAccOnCallEscalationPolicyDataSourceBothConfig("00000000-0000-0000-0000-000000000000", policyName),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config:      testAccOnCallEscalationPolicyDataSourceByNameConfig(acctest.RandomWithPrefix("tf-acc-missing")),
				ExpectError: regexp.MustCompile(`On-Call Escalation Policy Not Found`),
			},
			{
				Config: testAccOnCallEscalationPolicyDataSourceConfig(teamName, policyName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValuePairs(byUUID, tfjsonpath.New("id"), policy, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.CompareValuePairs(byUUID, tfjsonpath.New("uuid"), policy, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(byUUID, tfjsonpath.New("name"), knownvalue.StringExact(policyName)),
					statecheck.ExpectKnownValue(byUUID, tfjsonpath.New("timezone"), knownvalue.StringExact("Asia/Tokyo")),
					statecheck.ExpectKnownValue(byUUID, tfjsonpath.New("call_depth"), knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue(byUUID, tfjsonpath.New("call_traversal"), knownvalue.StringExact("breadth_first")),
					statecheck.ExpectKnownValue(byUUID, tfjsonpath.New("steps"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"uuid":              knownvalue.NotNull(),
							"position":          knownvalue.Int64Exact(0),
							"step_type":         knownvalue.StringExact("primary"),
							"simultaneous_call": knownvalue.Bool(false),
							"team": knownvalue.ObjectExact(map[string]knownvalue.Check{
								"uuid":  knownvalue.NotNull(),
								"name":  knownvalue.StringExact(teamName),
								"color": knownvalue.StringExact("FF0000"),
							}),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"uuid":              knownvalue.NotNull(),
							"position":          knownvalue.Int64Exact(1),
							"step_type":         knownvalue.StringExact("fallback"),
							"simultaneous_call": knownvalue.Bool(true),
							"team":              knownvalue.NotNull(),
						}),
					})),
					statecheck.CompareValuePairs(byUUID, stepTeamUUID(0), team, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.CompareValuePairs(byUUID, stepTeamUUID(1), team, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(byUUID, tfjsonpath.New("time_slots"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"uuid":        knownvalue.NotNull(),
							"day_of_week": knownvalue.Int64Exact(1),
							"start_time":  knownvalue.StringExact("09:00"),
							"end_time":    knownvalue.StringExact("18:00"),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"uuid":        knownvalue.NotNull(),
							"day_of_week": knownvalue.Int64Exact(3),
							"start_time":  knownvalue.StringExact("22:00"),
							"end_time":    knownvalue.StringExact("02:00"),
						}),
					})),
					statecheck.CompareValuePairs(byName, tfjsonpath.New("id"), policy, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.CompareValuePairs(byName, tfjsonpath.New("uuid"), policy, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.CompareValuePairs(byName, tfjsonpath.New("steps"), byUUID, tfjsonpath.New("steps"), compare.ValuesSame()),
					statecheck.CompareValuePairs(byName, tfjsonpath.New("time_slots"), byUUID, tfjsonpath.New("time_slots"), compare.ValuesSame()),
				},
			},
		},
	})
}

// testAccOnCallEscalationPolicyDataSourceConfig declares an on-call team and an escalation policy with two steps for it,
// and looks the policy up by uuid and by name.
func testAccOnCallEscalationPolicyDataSourceConfig(teamName, policyName string) string {
	return fmt.Sprintf(`
resource "waroom_on_call_team" "test" {
  name  = %[1]q
  color = "FF0000"

  rotations = [{
    timezone           = "Asia/Tokyo"
    started_at         = "2026-01-05T09:00:00+09:00"
    shift_handoff_time = "09:00"
    shift_handoff_wday = 1
  }]
}

resource "waroom_on_call_escalation_policy" "test" {
  name           = %[2]q
  timezone       = "Asia/Tokyo"
  call_depth     = 2
  call_traversal = "breadth_first"

  steps = [
    { team_uuid = waroom_on_call_team.test.uuid },
    { team_uuid = waroom_on_call_team.test.uuid, step_type = "fallback", simultaneous_call = true },
  ]

  time_slots = [
    { day_of_week = 1, start_time = "09:00", end_time = "18:00" },
    { day_of_week = 3, start_time = "22:00", end_time = "02:00" },
  ]
}

data "waroom_on_call_escalation_policy" "by_uuid" {
  uuid = waroom_on_call_escalation_policy.test.uuid
}

data "waroom_on_call_escalation_policy" "by_name" {
  name = waroom_on_call_escalation_policy.test.name
}
`, teamName, policyName)
}

func testAccOnCallEscalationPolicyDataSourceByNameConfig(name string) string {
	return fmt.Sprintf(`
data "waroom_on_call_escalation_policy" "test" {
  name = %[1]q
}
`, name)
}

func testAccOnCallEscalationPolicyDataSourceBothConfig(uuid, name string) string {
	return fmt.Sprintf(`
data "waroom_on_call_escalation_policy" "test" {
  uuid = %[1]q
  name = %[2]q
}
`, uuid, name)
}
