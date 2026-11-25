package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"
)

// E2E_WAROOM_ON_CALL_USER_SUB is the sub of an existing user with a verified phone number.
// Only such users can join a rotation, and there is no API to verify one, so the participant
// checks run only when it is set.
const onCallUserSubEnvName = "E2E_WAROOM_ON_CALL_USER_SUB"

func TestValidateOnCallTeamShiftHandoffWday(t *testing.T) {
	cases := []struct {
		name     string
		interval types.Int64
		wday     types.Int64
		wantErr  bool
	}{
		{"default interval with wday", types.Int64Null(), types.Int64Value(1), false},
		{"default interval without wday", types.Int64Null(), types.Int64Null(), true},
		{"weekly with wday", types.Int64Value(7), types.Int64Value(0), false},
		{"biweekly without wday", types.Int64Value(14), types.Int64Null(), true},
		{"daily without wday", types.Int64Value(1), types.Int64Null(), false},
		{"every 3 days with wday", types.Int64Value(3), types.Int64Value(2), true},
		{"unknown interval", types.Int64Unknown(), types.Int64Null(), false},
		{"unknown wday", types.Int64Value(3), types.Int64Unknown(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateOnCallTeamShiftHandoffWday(tc.interval, tc.wday)
			if (got != "") != tc.wantErr {
				t.Errorf("validateOnCallTeamShiftHandoffWday(%v, %v) = %q, wantErr %v", tc.interval, tc.wday, got, tc.wantErr)
			}
		})
	}
}

func TestOnCallTeamStartedAt(t *testing.T) {
	const rotationUUID = "660e8400-e29b-41d4-a716-446655440001"
	const configured = "2026-01-08T00:30:00+09:00"
	normalized := time.Date(2026, 1, 5, 10, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	changed := normalized.AddDate(0, 0, 7)

	prior := &OnCallTeamRotationModel{UUID: types.StringValue(rotationUUID), StartedAt: types.StringValue(configured)}
	otherPrior := &OnCallTeamRotationModel{UUID: types.StringValue("other"), StartedAt: types.StringValue(configured)}
	unknownPrior := &OnCallTeamRotationModel{UUID: types.StringUnknown(), StartedAt: types.StringValue(configured)}
	recorded := map[string]time.Time{rotationUUID: normalized.UTC()}

	cases := []struct {
		name       string
		api        time.Time
		prior      *OnCallTeamRotationModel
		recorded   map[string]time.Time
		afterWrite bool
		want       string
	}{
		{"after write keeps the planned value", normalized, unknownPrior, nil, true, configured},
		{"read keeps the configured value while the API value is unchanged", normalized, prior, recorded, false, configured},
		{"read uses the API value when it changed outside Terraform", changed, prior, recorded, false, changed.Format(time.RFC3339)},
		{"read uses the API value when nothing was recorded", normalized, prior, nil, false, "2026-01-05T10:00:00+09:00"},
		{"read keeps a value equal to the API value without a record", normalized, &OnCallTeamRotationModel{UUID: types.StringValue(rotationUUID), StartedAt: types.StringValue("2026-01-05T01:00:00Z")}, nil, false, "2026-01-05T01:00:00Z"},
		{"read uses the API value for another rotation", normalized, otherPrior, recorded, false, "2026-01-05T10:00:00+09:00"},
		{"no prior rotation (import)", normalized, nil, nil, false, "2026-01-05T10:00:00+09:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := onCallTeamStartedAt(rotationUUID, tc.api, tc.prior, tc.recorded, tc.afterWrite); got != tc.want {
				t.Errorf("onCallTeamStartedAt() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A rotation that is unknown as a whole at plan time is skipped, while the known ones are still checked.
func TestOnCallTeamResource_ValidateConfigUnknownRotation(t *testing.T) {
	ctx := context.Background()
	r := &OnCallTeamResource{}
	var schemaResp fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)
	s := schemaResp.Schema

	teamType := s.Type().TerraformType(ctx).(tftypes.Object)
	listType := teamType.AttributeTypes["rotations"].(tftypes.List)
	rotType := listType.ElementType.(tftypes.Object)

	rotationAttrs := map[string]tftypes.Value{}
	for name, typ := range rotType.AttributeTypes {
		rotationAttrs[name] = tftypes.NewValue(typ, nil)
	}
	rotationAttrs["timezone"] = tftypes.NewValue(tftypes.String, "Asia/Tokyo")
	rotationAttrs["started_at"] = tftypes.NewValue(tftypes.String, "2026-01-05T10:00:00+09:00")
	rotationAttrs["shift_handoff_time"] = tftypes.NewValue(tftypes.String, "10:00")
	// interval_days defaults to 7, so shift_handoff_wday is missing.
	invalid := tftypes.NewValue(rotType, rotationAttrs)

	raw := tftypes.NewValue(teamType, map[string]tftypes.Value{
		"id":    tftypes.NewValue(tftypes.String, nil),
		"uuid":  tftypes.NewValue(tftypes.String, nil),
		"name":  tftypes.NewValue(tftypes.String, "SRE"),
		"color": tftypes.NewValue(tftypes.String, "FF0000"),
		"rotations": tftypes.NewValue(listType, []tftypes.Value{
			tftypes.NewValue(rotType, tftypes.UnknownValue),
			invalid,
		}),
	})

	var resp fwresource.ValidateConfigResponse
	r.ValidateConfig(ctx, fwresource.ValidateConfigRequest{Config: tfsdk.Config{Schema: s, Raw: raw}}, &resp)

	errs := resp.Diagnostics.Errors()
	if len(errs) != 1 || !strings.Contains(errs[0].Detail(), "shift_handoff_wday is required") {
		t.Fatalf("want only the shift_handoff_wday error for rotations[1], got: %v", resp.Diagnostics)
	}
}

// In read, prior rotations are matched by UUID, so a rotation added outside Terraform in front of the
// existing ones does not make them lose their configured started_at.
func TestOnCallTeamStateFromAPI_ReadMatchesRotationsByUUID(t *testing.T) {
	const uuidA, uuidB, uuidNew = "00000000-0000-4000-8000-00000000000a", "00000000-0000-4000-8000-00000000000b", "00000000-0000-4000-8000-0000000000ff"
	rotation := func(uuid string, position int) string {
		return fmt.Sprintf(`{"uuid":%q,"position":%d,"timezone":"Asia/Tokyo","started_at":"2026-01-05T10:00:00+09:00",
			"interval_days":7,"shift_handoff_wday":1,"shift_handoff_time":"10:00","time_slots":[],"participants":[],"on_duty_participant":null}`, uuid, position)
	}
	body := fmt.Sprintf(`{"uuid":"00000000-0000-4000-8000-000000000001","name":"SRE","color":"FF0000","rotation":null,
		"rotations":[%s,%s,%s],"on_duty_participant":null}`, rotation(uuidNew, 0), rotation(uuidA, 1), rotation(uuidB, 2))
	var team waroom.OnCallTeam
	if err := json.Unmarshal([]byte(body), &team); err != nil {
		t.Fatalf("unmarshal team: %s", err)
	}

	const configuredA, configuredB = "2026-01-08T00:30:00+09:00", "2026-01-06T12:00:00+09:00"
	prior := []OnCallTeamRotationModel{
		{UUID: types.StringValue(uuidA), StartedAt: types.StringValue(configuredA)},
		{UUID: types.StringValue(uuidB), StartedAt: types.StringValue(configuredB)},
	}
	recorded := map[string]time.Time{uuidA: team.Rotations[1].StartedAt, uuidB: team.Rotations[2].StartedAt}

	state, _, diags := onCallTeamStateFromAPI(context.Background(), &team, prior, recorded, false)
	if diags.HasError() {
		t.Fatalf("onCallTeamStateFromAPI: %v", diags)
	}
	want := []string{"2026-01-05T10:00:00+09:00", configuredA, configuredB}
	for i, w := range want {
		if got := state.Rotations[i].StartedAt.ValueString(); got != w {
			t.Errorf("rotations[%d].started_at = %q, want %q", i, got, w)
		}
	}
}

func TestAccOnCallTeamResource(t *testing.T) {
	const res = "waroom_on_call_team.test"
	name := acctest.RandomWithPrefix("tf-acc")
	updatedName := name + "-updated"
	userSub := os.Getenv(onCallUserSubEnvName)
	if userSub == "" {
		t.Logf("%s is not set; participant_user_subs is tested only with an empty list", onCallUserSubEnvName)
	}

	rotation0UUID := statecheck.CompareValue(compare.ValuesSame())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOnCallTeamDestroy(t),
		Steps: []resource.TestStep{
			// shift_handoff_wday must fit interval_days
			{
				Config: testAccOnCallTeamResourceConfig(name, "1E90FF", `
  rotations = [{
    timezone           = "Asia/Tokyo"
    started_at         = "2026-01-05T10:00:00+09:00"
    shift_handoff_time = "10:00"
  }]`),
				ExpectError: regexp.MustCompile(`shift_handoff_wday is required`),
			},
			// Create with one weekly rotation, whose started_at is already a handoff
			{
				Config: testAccOnCallTeamResourceConfig(name, "1E90FF", `
  rotations = [{
    timezone           = "Asia/Tokyo"
    started_at         = "2026-01-05T10:00:00+09:00"
    shift_handoff_time = "10:00"
    shift_handoff_wday = 1
    time_slots = [
      { day_of_week = 3, start_time = "09:00", end_time = "18:00" },
      { day_of_week = 1, start_time = "22:00", end_time = "02:00" },
    ]
  }]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, tfjsonpath.New("uuid"), knownvalue.NotNull()),
					statecheck.CompareValuePairs(res, tfjsonpath.New("id"), res, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(res, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(res, tfjsonpath.New("color"), knownvalue.StringExact("1E90FF")),
					statecheck.ExpectKnownValue(res, tfjsonpath.New("rotations"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"uuid":                  knownvalue.NotNull(),
							"timezone":              knownvalue.StringExact("Asia/Tokyo"),
							"started_at":            knownvalue.StringExact("2026-01-05T10:00:00+09:00"),
							"shift_handoff_time":    knownvalue.StringExact("10:00"),
							"shift_handoff_wday":    knownvalue.Int64Exact(1),
							"interval_days":         knownvalue.Int64Exact(7),
							"participant_user_subs": knownvalue.ListSizeExact(0),
							"time_slots": knownvalue.SetExact([]knownvalue.Check{
								knownvalue.ObjectExact(map[string]knownvalue.Check{
									"day_of_week": knownvalue.Int64Exact(1),
									"start_time":  knownvalue.StringExact("22:00"),
									"end_time":    knownvalue.StringExact("02:00"),
								}),
								knownvalue.ObjectExact(map[string]knownvalue.Check{
									"day_of_week": knownvalue.Int64Exact(3),
									"start_time":  knownvalue.StringExact("09:00"),
									"end_time":    knownvalue.StringExact("18:00"),
								}),
							}),
						}),
					})),
					rotation0UUID.AddStateValue(res, tfjsonpath.New("rotations").AtSliceIndex(0).AtMapKey("uuid")),
				},
			},
			// Import
			{
				ResourceName:      res,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Import with a non-canonical UUID, which is normalized
			{
				ResourceName:      res,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return strings.ToUpper(s.RootModule().Resources[res].Primary.Attributes["uuid"]), nil
				},
			},
			// Import with an ID that is not a UUID
			{
				ResourceName:  res,
				ImportState:   true,
				ImportStateId: name,
				ExpectError:   regexp.MustCompile(`Invalid Import ID`),
			},
			// Update the team, change the first rotation (with a started_at that Waroom adjusts) and add a second one
			{
				Config: testAccOnCallTeamResourceConfig(updatedName, "FF8C00", fmt.Sprintf(`
  rotations = [
    {
      timezone              = "Asia/Tokyo"
      started_at            = "2026-01-08T00:30:00+09:00"
      shift_handoff_time    = "09:00"
      shift_handoff_wday    = 4
      interval_days         = 14
      participant_user_subs = %s
      time_slots = [
        { day_of_week = 5, start_time = "00:00", end_time = "23:59" },
      ]
    },
    {
      timezone           = "America/New_York"
      started_at         = "2026-01-07T23:00:00-05:00"
      shift_handoff_time = "09:30"
      interval_days      = 3
    },
  ]`, testAccOnCallTeamParticipants(userSub))),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, tfjsonpath.New("name"), knownvalue.StringExact(updatedName)),
					statecheck.ExpectKnownValue(res, tfjsonpath.New("color"), knownvalue.StringExact("FF8C00")),
					statecheck.ExpectKnownValue(res, tfjsonpath.New("rotations"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"uuid":                  knownvalue.NotNull(),
							"timezone":              knownvalue.StringExact("Asia/Tokyo"),
							"started_at":            knownvalue.StringExact("2026-01-08T00:30:00+09:00"),
							"shift_handoff_time":    knownvalue.StringExact("09:00"),
							"shift_handoff_wday":    knownvalue.Int64Exact(4),
							"interval_days":         knownvalue.Int64Exact(14),
							"participant_user_subs": testAccOnCallTeamParticipantsCheck(userSub),
							"time_slots": knownvalue.SetExact([]knownvalue.Check{
								knownvalue.ObjectExact(map[string]knownvalue.Check{
									"day_of_week": knownvalue.Int64Exact(5),
									"start_time":  knownvalue.StringExact("00:00"),
									"end_time":    knownvalue.StringExact("23:59"),
								}),
							}),
						}),
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"uuid":                  knownvalue.NotNull(),
							"timezone":              knownvalue.StringExact("America/New_York"),
							"started_at":            knownvalue.StringExact("2026-01-07T23:00:00-05:00"),
							"shift_handoff_time":    knownvalue.StringExact("09:30"),
							"shift_handoff_wday":    knownvalue.Null(),
							"interval_days":         knownvalue.Int64Exact(3),
							"participant_user_subs": knownvalue.ListSizeExact(0),
							"time_slots":            knownvalue.SetSizeExact(0),
						}),
					})),
					// The first rotation is updated in place, not recreated.
					rotation0UUID.AddStateValue(res, tfjsonpath.New("rotations").AtSliceIndex(0).AtMapKey("uuid")),
				},
			},
			// Remove the second rotation and the participants
			{
				Config: testAccOnCallTeamResourceConfig(updatedName, "FF8C00", `
  rotations = [{
    timezone           = "Asia/Tokyo"
    started_at         = "2026-01-08T00:30:00+09:00"
    shift_handoff_time = "09:00"
    shift_handoff_wday = 4
    interval_days      = 14
  }]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(res, tfjsonpath.New("rotations"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue(res, tfjsonpath.New("rotations").AtSliceIndex(0).AtMapKey("participant_user_subs"), knownvalue.ListSizeExact(0)),
					statecheck.ExpectKnownValue(res, tfjsonpath.New("rotations").AtSliceIndex(0).AtMapKey("time_slots"), knownvalue.SetSizeExact(0)),
					rotation0UUID.AddStateValue(res, tfjsonpath.New("rotations").AtSliceIndex(0).AtMapKey("uuid")),
				},
			},
		},
	})
}

func testAccOnCallTeamParticipants(userSub string) string {
	if userSub == "" {
		return "[]"
	}
	return fmt.Sprintf("[%q]", userSub)
}

func testAccOnCallTeamParticipantsCheck(userSub string) knownvalue.Check {
	if userSub == "" {
		return knownvalue.ListSizeExact(0)
	}
	return knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact(userSub)})
}

func testAccCheckOnCallTeamDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccWaroomClient(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_on_call_team" {
				continue
			}
			id := rs.Primary.Attributes["uuid"]
			_, err := client.OnCallTeams.Get(context.Background(), waroom.GetOnCallTeamParams{UUID: id})
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				continue
			}
			if err != nil {
				return fmt.Errorf("get on-call team %s: %w", id, err)
			}
			return fmt.Errorf("on-call team %s still exists", id)
		}
		return nil
	}
}

func testAccOnCallTeamResourceConfig(name, color, rotations string) string {
	return fmt.Sprintf(`
resource "waroom_on_call_team" "test" {
  name  = %q
  color = %q
%s
}
`, name, color, strings.TrimPrefix(rotations, "\n"))
}
