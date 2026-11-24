package provider

import (
	"context"
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

	"github.com/topotal/terraform-provider-waroom/internal/utils"
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

	// There is no on-call resource in this provider yet, so the prerequisites are created with the client.
	client := testAccOnCallEscalationPolicyClient(t)
	teamName := acctest.RandomWithPrefix("tf-acc")
	policyName := acctest.RandomWithPrefix("tf-acc")
	team := testAccEscalationPolicyCreateTeam(t, client, teamName)
	policy := testAccEscalationPolicyCreatePolicy(t, client, policyName, team.UUID)

	const byUUID = "data.waroom_on_call_escalation_policy.by_uuid"
	const byName = "data.waroom_on_call_escalation_policy.by_name"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Error cases first: the post-test destroy runs with the last step's config, which must be valid.
			{
				Config:      testAccOnCallEscalationPolicyDataSourceBothConfig(policy.UUID, policyName),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config:      testAccOnCallEscalationPolicyDataSourceByNameConfig(acctest.RandomWithPrefix("tf-acc-missing")),
				ExpectError: regexp.MustCompile(`On-Call Escalation Policy Not Found`),
			},
			{
				Config: testAccOnCallEscalationPolicyDataSourceConfig(policy.UUID, policyName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(byUUID, tfjsonpath.New("id"), knownvalue.StringExact(policy.UUID)),
					statecheck.ExpectKnownValue(byUUID, tfjsonpath.New("uuid"), knownvalue.StringExact(policy.UUID)),
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
								"uuid":  knownvalue.StringExact(team.UUID),
								"name":  knownvalue.StringExact(teamName),
								"color": knownvalue.StringExact("FF0000"),
							}),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"uuid":              knownvalue.NotNull(),
							"position":          knownvalue.Int64Exact(1),
							"step_type":         knownvalue.StringExact("fallback"),
							"simultaneous_call": knownvalue.Bool(true),
							"team": knownvalue.ObjectPartial(map[string]knownvalue.Check{
								"uuid": knownvalue.StringExact(team.UUID),
							}),
						}),
					})),
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
					statecheck.ExpectKnownValue(byName, tfjsonpath.New("id"), knownvalue.StringExact(policy.UUID)),
					statecheck.ExpectKnownValue(byName, tfjsonpath.New("uuid"), knownvalue.StringExact(policy.UUID)),
					statecheck.CompareValuePairs(byName, tfjsonpath.New("steps"), byUUID, tfjsonpath.New("steps"), compare.ValuesSame()),
					statecheck.CompareValuePairs(byName, tfjsonpath.New("time_slots"), byUUID, tfjsonpath.New("time_slots"), compare.ValuesSame()),
				},
			},
		},
	})
}

func testAccOnCallEscalationPolicyDataSourceConfig(uuid, name string) string {
	return fmt.Sprintf(`
data "waroom_on_call_escalation_policy" "by_uuid" {
  uuid = %[1]q
}

data "waroom_on_call_escalation_policy" "by_name" {
  name = %[2]q
}
`, uuid, name)
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

// testAccOnCallEscalationPolicyClient builds a client from the same environment variables as the provider.
func testAccOnCallEscalationPolicyClient(t *testing.T) *waroom.Client {
	t.Helper()
	client, err := waroom.NewClient(context.Background(),
		waroom.WithBaseURL(os.Getenv(utils.WaroomAPIEndPointEnvName)),
		waroom.WithAPIKey(os.Getenv(utils.WaroomAPIKeyEnvName)),
	)
	if err != nil {
		t.Fatalf("creating Waroom client: %v", err)
	}
	return client
}

// testAccEscalationPolicyCreateTeam creates an on-call team with one rotation without participants and deletes it on cleanup.
func testAccEscalationPolicyCreateTeam(t *testing.T, client *waroom.Client, name string) *waroom.CreatedOnCallTeam {
	t.Helper()
	rotation := waroom.CreateOnCallTeamRotationReq{
		Timezone:         "Asia/Tokyo",
		StartedAt:        "2026-01-05T09:00:00+09:00",
		ShiftHandoffTime: "09:00",
	}
	rotation.ShiftHandoffWday.SetTo(1)
	team, err := client.OnCallTeams.Create(context.Background(), waroom.CreateOnCallTeamReq{
		Name:      name,
		Color:     "FF0000",
		Rotations: []waroom.CreateOnCallTeamRotationReq{rotation},
	})
	if err != nil {
		t.Fatalf("creating on-call team: %v", err)
	}
	t.Cleanup(func() {
		if err := client.OnCallTeams.Delete(context.Background(), waroom.DeleteOnCallTeamParams{UUID: team.UUID}); err != nil {
			t.Errorf("deleting on-call team %s: %v", team.UUID, err)
		}
	})
	return team
}

// testAccEscalationPolicyCreatePolicy creates an escalation policy with two steps for the team and deletes it on cleanup.
func testAccEscalationPolicyCreatePolicy(t *testing.T, client *waroom.Client, name, teamUUID string) *waroom.CreatedOnCallEscalationPolicy {
	t.Helper()
	fallback := waroom.CreateOnCallEscalationPolicyStep{TeamUUID: teamUUID}
	fallback.StepType.SetTo("fallback")
	fallback.SimultaneousCall.SetTo(true)
	req := waroom.CreateOnCallEscalationPolicyReq{
		Name:     name,
		Timezone: waroom.NewOptString("Asia/Tokyo"),
		Steps: []waroom.CreateOnCallEscalationPolicyStep{
			{TeamUUID: teamUUID},
			fallback,
		},
		TimeSlots: []waroom.CreateOnCallEscalationPolicyTimeSlot{
			{DayOfWeek: 1, StartTime: "09:00", EndTime: "18:00"},
			{DayOfWeek: 3, StartTime: "22:00", EndTime: "02:00"},
		},
	}
	req.CallDepth.SetTo(2)
	req.CallTraversal.SetTo("breadth_first")
	policy, err := client.OnCallEscalationPolicies.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("creating on-call escalation policy: %v", err)
	}
	t.Cleanup(func() {
		if err := client.OnCallEscalationPolicies.Delete(context.Background(), waroom.DeleteOnCallEscalationPolicyParams{UUID: policy.UUID}); err != nil {
			t.Errorf("deleting on-call escalation policy %s: %v", policy.UUID, err)
		}
	})
	return policy
}
