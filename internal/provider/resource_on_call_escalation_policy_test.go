package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	waroom "github.com/topotal/waroom-client-go/waroom"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
)

func TestNewOnCallEscalationPolicyResourceModel(t *testing.T) {
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

	got, diags := newOnCallEscalationPolicyResourceModel(&policy)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	want := OnCallEscalationPolicyResourceModel{
		Id:            types.StringValue("770e8400-e29b-41d4-a716-446655440001"),
		UUID:          types.StringValue("770e8400-e29b-41d4-a716-446655440001"),
		Name:          types.StringValue("Primary escalation"),
		Timezone:      types.StringValue("Asia/Tokyo"),
		CallDepth:     types.Int64Value(2),
		CallTraversal: types.StringValue("breadth_first"),
		Steps: types.ListValueMust(types.ObjectType{AttrTypes: onCallEscalationPolicyStepAttrTypes}, []attr.Value{
			types.ObjectValueMust(onCallEscalationPolicyStepAttrTypes, map[string]attr.Value{
				"team_uuid":         types.StringValue("880e8400-e29b-41d4-a716-446655440001"),
				"step_type":         types.StringValue("fallback"),
				"simultaneous_call": types.BoolValue(true),
			}),
		}),
		TimeSlots: types.SetValueMust(types.ObjectType{AttrTypes: onCallEscalationPolicyTimeSlotAttrTypes}, []attr.Value{
			types.ObjectValueMust(onCallEscalationPolicyTimeSlotAttrTypes, map[string]attr.Value{
				"day_of_week": types.Int64Value(1),
				"start_time":  types.StringValue("22:00"),
				"end_time":    types.StringValue("02:00"),
			}),
		}),
	}
	if !got.Id.Equal(want.Id) || !got.UUID.Equal(want.UUID) || !got.Name.Equal(want.Name) ||
		!got.Timezone.Equal(want.Timezone) || !got.CallDepth.Equal(want.CallDepth) ||
		!got.CallTraversal.Equal(want.CallTraversal) || !got.Steps.Equal(want.Steps) || !got.TimeSlots.Equal(want.TimeSlots) {
		t.Errorf("model mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNewOnCallEscalationPolicyResourceModel_Empty(t *testing.T) {
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

	got, diags := newOnCallEscalationPolicyResourceModel(&policy)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !got.CallDepth.IsNull() {
		t.Errorf("call_depth: want null, got %s", got.CallDepth)
	}
	// Empty (not null) so that the state matches the default of steps.
	if got.Steps.IsNull() || len(got.Steps.Elements()) != 0 {
		t.Errorf("steps: want empty list, got %s", got.Steps)
	}
	if got.TimeSlots.IsNull() || len(got.TimeSlots.Elements()) != 0 {
		t.Errorf("time_slots: want empty set, got %s", got.TimeSlots)
	}
}

func TestToOnCallEscalationPolicy(t *testing.T) {
	var created waroom.CreatedOnCallEscalationPolicy
	if err := created.UnmarshalJSON([]byte(`{
  "uuid": "770e8400-e29b-41d4-a716-446655440001",
  "name": "Primary escalation",
  "timezone": "UTC",
  "call_depth": null,
  "call_traversal": "depth_first",
  "steps": [
    {
      "uuid": "770e8400-e29b-41d4-a716-446655440011",
      "position": 0,
      "step_type": "primary",
      "simultaneous_call": false,
      "team": {"uuid": "880e8400-e29b-41d4-a716-446655440001", "name": "SRE", "color": "FF0000"}
    }
  ],
  "time_slots": [
    {"uuid": "770e8400-e29b-41d4-a716-446655440021", "day_of_week": 0, "start_time": "00:00", "end_time": "23:59"}
  ]
}`)); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got, err := toOnCallEscalationPolicy(&created)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UUID != created.UUID || got.Timezone != "UTC" || !got.CallDepth.IsNull() ||
		len(got.Steps) != 1 || got.Steps[0].Team.UUID != created.Steps[0].Team.UUID ||
		len(got.TimeSlots) != 1 || got.TimeSlots[0].EndTime != "23:59" {
		t.Errorf("conversion mismatch: %#v", got)
	}
}

func TestOnCallEscalationPolicyTimeRegexp(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"00:00", true},
		{"09:30", true},
		{"23:59", true},
		{"24:00", false},
		{"9:00", false},
		{"09:60", false},
		{"09:00:00", false},
		{"", false},
	} {
		if got := onCallEscalationPolicyTimeRegexp.MatchString(tc.in); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestAccOnCallEscalationPolicyResource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	// Used only to look up the UUIDs of the steps / time slots, which the resource does not expose, and in CheckDestroy.
	client := testAccOnCallEscalationPolicyClient(t)
	team1Name := acctest.RandomWithPrefix("tf-acc")
	team2Name := acctest.RandomWithPrefix("tf-acc")
	name := acctest.RandomWithPrefix("tf-acc")
	updatedName := acctest.RandomWithPrefix("tf-acc")
	renamedName := acctest.RandomWithPrefix("tf-acc")
	updatedBody := `
  timezone       = "UTC"
  call_depth     = 2
  call_traversal = "breadth_first"

  steps = [
    { team_uuid = waroom_on_call_team.team2.uuid, step_type = "primary" },
    { team_uuid = waroom_on_call_team.team1.uuid, step_type = "fallback", simultaneous_call = true },
  ]

  time_slots = [
    { day_of_week = 0, start_time = "00:00", end_time = "23:59" },
  ]
`
	// The UUIDs of the steps and time slots after the update, to check that renaming does not recreate them.
	var childUUIDs []string

	const addr = "waroom_on_call_escalation_policy.test"
	const team1 = "waroom_on_call_team.team1"
	const team2 = "waroom_on_call_team.team2"
	stepTeamUUID := func(i int) tfjsonpath.Path {
		return tfjsonpath.New("steps").AtSliceIndex(i).AtMapKey("team_uuid")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOnCallEscalationPolicyDestroy(client),
		Steps: []resource.TestStep{
			// Create with defaults. time_slots are written out of the API's order to check they are a set.
			{
				Config: testAccOnCallEscalationPolicyResourceConfig(team1Name, team2Name, name, `
  steps = [
    { team_uuid = waroom_on_call_team.team1.uuid },
  ]

  time_slots = [
    { day_of_week = 3, start_time = "22:00", end_time = "02:00" },
    { day_of_week = 1, start_time = "09:00", end_time = "18:00" },
  ]
`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("uuid"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("timezone"), knownvalue.StringExact("Asia/Tokyo")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("call_depth"), knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("call_traversal"), knownvalue.StringExact("depth_first")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("steps"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"team_uuid":         knownvalue.NotNull(),
							"step_type":         knownvalue.StringExact("primary"),
							"simultaneous_call": knownvalue.Bool(false),
						}),
					})),
					statecheck.CompareValuePairs(addr, stepTeamUUID(0), team1, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("time_slots"), knownvalue.SetExact([]knownvalue.Check{
						testAccOnCallEscalationPolicyTimeSlot(1, "09:00", "18:00"),
						testAccOnCallEscalationPolicyTimeSlot(3, "22:00", "02:00"),
					})),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update every attribute.
			{
				Config: testAccOnCallEscalationPolicyResourceConfig(team1Name, team2Name, updatedName, updatedBody),
				Check:  testAccCaptureOnCallEscalationPolicyChildUUIDs(client, addr, &childUUIDs),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.StringExact(updatedName)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("timezone"), knownvalue.StringExact("UTC")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("call_depth"), knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("call_traversal"), knownvalue.StringExact("breadth_first")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("steps"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"team_uuid":         knownvalue.NotNull(),
							"step_type":         knownvalue.StringExact("primary"),
							"simultaneous_call": knownvalue.Bool(false),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"team_uuid":         knownvalue.NotNull(),
							"step_type":         knownvalue.StringExact("fallback"),
							"simultaneous_call": knownvalue.Bool(true),
						}),
					})),
					statecheck.CompareValuePairs(addr, stepTeamUUID(0), team2, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.CompareValuePairs(addr, stepTeamUUID(1), team1, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("time_slots"), knownvalue.SetExact([]knownvalue.Check{
						testAccOnCallEscalationPolicyTimeSlot(0, "00:00", "23:59"),
					})),
				},
			},
			// Changing only the name does not send steps / time_slots, so the API keeps them as they are.
			{
				Config: testAccOnCallEscalationPolicyResourceConfig(team1Name, team2Name, renamedName, updatedBody),
				Check:  testAccCheckOnCallEscalationPolicyChildUUIDs(client, addr, &childUUIDs),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.StringExact(renamedName)),
				},
			},
			// Removing the optional attributes resets them to their defaults.
			{
				Config: testAccOnCallEscalationPolicyResourceConfig(team1Name, team2Name, renamedName, `
  time_slots = [
    { day_of_week = 5, start_time = "18:00", end_time = "09:00" },
  ]
`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("timezone"), knownvalue.StringExact("Asia/Tokyo")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("call_depth"), knownvalue.Null()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("call_traversal"), knownvalue.StringExact("depth_first")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("steps"), knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("time_slots"), knownvalue.SetExact([]knownvalue.Check{
						testAccOnCallEscalationPolicyTimeSlot(5, "18:00", "09:00"),
					})),
				},
			},
		},
	})
}

// testAccOnCallEscalationPolicyResourceConfig declares two on-call teams (team1 and team2) for the steps
// and the escalation policy with the given name and body.
func testAccOnCallEscalationPolicyResourceConfig(team1Name, team2Name, name, body string) string {
	return fmt.Sprintf(`
resource "waroom_on_call_team" "team1" {
  name  = %[1]q
  color = "FF0000"

  rotations = [{
    timezone           = "Asia/Tokyo"
    started_at         = "2026-01-05T09:00:00+09:00"
    shift_handoff_time = "09:00"
    shift_handoff_wday = 1
  }]
}

resource "waroom_on_call_team" "team2" {
  # Creating two on-call teams at the same time makes the API return 500
  # (https://github.com/topotal/waroom-client-go/issues/210), so create them one by one.
  depends_on = [waroom_on_call_team.team1]
  name  = %[2]q
  color = "1E90FF"

  rotations = [{
    timezone           = "Asia/Tokyo"
    started_at         = "2026-01-05T09:00:00+09:00"
    shift_handoff_time = "09:00"
    shift_handoff_wday = 1
  }]
}

resource "waroom_on_call_escalation_policy" "test" {
  name = %[3]q
%[4]s
}
`, team1Name, team2Name, name, body)
}

func testAccOnCallEscalationPolicyTimeSlot(dayOfWeek int64, startTime, endTime string) knownvalue.Check {
	return knownvalue.ObjectExact(map[string]knownvalue.Check{
		"day_of_week": knownvalue.Int64Exact(dayOfWeek),
		"start_time":  knownvalue.StringExact(startTime),
		"end_time":    knownvalue.StringExact(endTime),
	})
}

// testAccOnCallEscalationPolicyChildUUIDs returns the UUIDs of the steps and time slots of the policy in the state.
func testAccOnCallEscalationPolicyChildUUIDs(client *waroom.Client, s *terraform.State, addr string) ([]string, error) {
	rs, ok := s.RootModule().Resources[addr]
	if !ok {
		return nil, fmt.Errorf("resource %s not found in state", addr)
	}
	policy, err := client.OnCallEscalationPolicies.Get(context.Background(), waroom.GetOnCallEscalationPolicyParams{UUID: rs.Primary.Attributes["uuid"]})
	if err != nil {
		return nil, err
	}
	var uuids []string
	for _, step := range policy.Steps {
		uuids = append(uuids, step.UUID)
	}
	for _, slot := range policy.TimeSlots {
		uuids = append(uuids, slot.UUID)
	}
	return uuids, nil
}

// testAccCaptureOnCallEscalationPolicyChildUUIDs stores the UUIDs of the steps and time slots in uuids.
func testAccCaptureOnCallEscalationPolicyChildUUIDs(client *waroom.Client, addr string, uuids *[]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got, err := testAccOnCallEscalationPolicyChildUUIDs(client, s, addr)
		if err != nil {
			return err
		}
		*uuids = got
		return nil
	}
}

// testAccCheckOnCallEscalationPolicyChildUUIDs checks that the steps and time slots still have the UUIDs in uuids.
func testAccCheckOnCallEscalationPolicyChildUUIDs(client *waroom.Client, addr string, uuids *[]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		got, err := testAccOnCallEscalationPolicyChildUUIDs(client, s, addr)
		if err != nil {
			return err
		}
		if len(*uuids) == 0 || fmt.Sprint(got) != fmt.Sprint(*uuids) {
			return fmt.Errorf("steps / time slots were recreated: before %v, after %v", *uuids, got)
		}
		return nil
	}
}

// testAccCheckOnCallEscalationPolicyDestroy checks that every escalation policy in the state is gone.
func testAccCheckOnCallEscalationPolicyDestroy(client *waroom.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_on_call_escalation_policy" {
				continue
			}
			_, err := client.OnCallEscalationPolicies.Get(context.Background(), waroom.GetOnCallEscalationPolicyParams{UUID: rs.Primary.Attributes["uuid"]})
			if err == nil {
				return fmt.Errorf("on-call escalation policy %s still exists", rs.Primary.Attributes["uuid"])
			}
			var apiErr *waroom.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
				return fmt.Errorf("checking on-call escalation policy %s: %w", rs.Primary.Attributes["uuid"], err)
			}
		}
		return nil
	}
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
