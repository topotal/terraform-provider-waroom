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
)

func TestNewOnCallTriggerResourceModel(t *testing.T) {
	var created waroom.CreatedOnCallTrigger
	if err := created.UnmarshalJSON([]byte(`{
  "uuid": "770e8400-e29b-41d4-a716-446655440001",
  "name": "critical incidents",
  "enabled": false,
  "event_type": "incident_created",
  "severity_filters": ["critical", "high"],
  "source_filters": ["integration", "web"],
  "retry_count": 2,
  "service": {"name": "svc"},
  "escalation_policies": [
    {"uuid": "880e8400-e29b-41d4-a716-446655440001", "name": "primary"},
    {"uuid": "880e8400-e29b-41d4-a716-446655440002", "name": "secondary"}
  ]
}`)); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	trigger, err := toOnCallTrigger(&created)
	if err != nil {
		t.Fatalf("toOnCallTrigger: %v", err)
	}
	got, diags := newOnCallTriggerResourceModel(trigger)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	stringSet := func(values ...string) types.Set {
		elems := make([]attr.Value, 0, len(values))
		for _, v := range values {
			elems = append(elems, types.StringValue(v))
		}
		return types.SetValueMust(types.StringType, elems)
	}
	want := OnCallTriggerResourceModel{
		Id:          types.StringValue("770e8400-e29b-41d4-a716-446655440001"),
		UUID:        types.StringValue("770e8400-e29b-41d4-a716-446655440001"),
		Name:        types.StringValue("critical incidents"),
		ServiceName: types.StringValue("svc"),
		EscalationPolicyUUIDs: stringSet(
			"880e8400-e29b-41d4-a716-446655440002", // a set: the order does not matter
			"880e8400-e29b-41d4-a716-446655440001",
		),
		SeverityFilters: stringSet("high", "critical"),
		SourceFilters:   stringSet("integration", "web"),
		Enabled:         types.BoolValue(false),
		EventType:       types.StringValue("incident_created"),
		RetryCount:      types.Int64Value(2),
	}
	if !got.Id.Equal(want.Id) || !got.UUID.Equal(want.UUID) || !got.Name.Equal(want.Name) ||
		!got.ServiceName.Equal(want.ServiceName) || !got.EscalationPolicyUUIDs.Equal(want.EscalationPolicyUUIDs) ||
		!got.SeverityFilters.Equal(want.SeverityFilters) || !got.SourceFilters.Equal(want.SourceFilters) ||
		!got.Enabled.Equal(want.Enabled) || !got.EventType.Equal(want.EventType) || !got.RetryCount.Equal(want.RetryCount) {
		t.Errorf("model mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestNewOnCallTriggerResourceModel_Empty(t *testing.T) {
	got, diags := newOnCallTriggerResourceModel(&waroom.OnCallTrigger{UUID: "x", Name: "n"})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// Empty API arrays become empty sets, not null, so that the state matches an empty configuration.
	for name, s := range map[string]types.Set{
		"escalation_policy_uuids": got.EscalationPolicyUUIDs,
		"severity_filters":        got.SeverityFilters,
		"source_filters":          got.SourceFilters,
	} {
		if s.IsNull() || len(s.Elements()) != 0 {
			t.Errorf("%s = %v, want an empty set", name, s)
		}
	}
}

func TestAccOnCallTriggerResource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	client := testAccWaroomClient(t)
	services := testAccServices(t, 2)
	policy1Name := acctest.RandomWithPrefix("tf-acc")
	policy2Name := acctest.RandomWithPrefix("tf-acc")
	name := acctest.RandomWithPrefix("tf-acc")
	updatedName := acctest.RandomWithPrefix("tf-acc")

	const addr = "waroom_on_call_trigger.test"
	const policy1 = "waroom_on_call_escalation_policy.policy1"
	const policy2 = "waroom_on_call_escalation_policy.policy2"
	stringSet := func(values ...string) knownvalue.Check {
		checks := make([]knownvalue.Check, 0, len(values))
		for _, v := range values {
			checks = append(checks, knownvalue.StringExact(v))
		}
		return knownvalue.SetExact(checks)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOnCallTriggerDestroy(client),
		Steps: []resource.TestStep{
			// Create with defaults. Disabled so that the test never pages anyone.
			{
				Config: testAccOnCallTriggerResourceConfig(policy1Name, policy2Name, name, services[0], `
  escalation_policy_uuids = [waroom_on_call_escalation_policy.policy1.uuid]
  severity_filters        = ["critical", "high"]
  enabled                 = false
`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("uuid"), knownvalue.NotNull()),
					statecheck.CompareValuePairs(addr, tfjsonpath.New("id"), addr, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("service_name"), knownvalue.StringExact(services[0])),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("escalation_policy_uuids"), knownvalue.SetSizeExact(1)),
					statecheck.CompareValueCollection(addr, []tfjsonpath.Path{tfjsonpath.New("escalation_policy_uuids")}, policy1, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("severity_filters"), stringSet("critical", "high")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("source_filters"), stringSet("integration")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("event_type"), knownvalue.StringExact("incident_created")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("retry_count"), knownvalue.Int64Exact(0)),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update every attribute. The trigger is enabled only in this step; the test creates no incidents.
			{
				Config: testAccOnCallTriggerResourceConfig(policy1Name, policy2Name, updatedName, services[1], `
  escalation_policy_uuids = [waroom_on_call_escalation_policy.policy2.uuid, waroom_on_call_escalation_policy.policy1.uuid]
  severity_filters        = ["low", "info", "unknown"]
  source_filters          = ["slack", "web"]
  enabled                 = true
  event_type              = "incident_created"
  retry_count             = 3
`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("name"), knownvalue.StringExact(updatedName)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("service_name"), knownvalue.StringExact(services[1])),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("escalation_policy_uuids"), knownvalue.SetSizeExact(2)),
					statecheck.CompareValueCollection(addr, []tfjsonpath.Path{tfjsonpath.New("escalation_policy_uuids")}, policy1, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.CompareValueCollection(addr, []tfjsonpath.Path{tfjsonpath.New("escalation_policy_uuids")}, policy2, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("severity_filters"), stringSet("low", "info", "unknown")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("source_filters"), stringSet("slack", "web")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("enabled"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("retry_count"), knownvalue.Int64Exact(3)),
				},
			},
			// Removing the optional attributes resets them to their defaults (except enabled, kept disabled).
			{
				Config: testAccOnCallTriggerResourceConfig(policy1Name, policy2Name, updatedName, services[1], `
  escalation_policy_uuids = [waroom_on_call_escalation_policy.policy2.uuid]
  severity_filters        = ["critical"]
  enabled                 = false
`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("escalation_policy_uuids"), knownvalue.SetSizeExact(1)),
					statecheck.CompareValueCollection(addr, []tfjsonpath.Path{tfjsonpath.New("escalation_policy_uuids")}, policy2, tfjsonpath.New("uuid"), compare.ValuesSame()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("severity_filters"), stringSet("critical")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("source_filters"), stringSet("integration")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("enabled"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("retry_count"), knownvalue.Int64Exact(0)),
				},
			},
		},
	})
}

// testAccOnCallTriggerResourceConfig declares two escalation policies (policy1 and policy2)
// and the trigger with the given name, service and body.
func testAccOnCallTriggerResourceConfig(policy1Name, policy2Name, name, serviceName, body string) string {
	return fmt.Sprintf(`
resource "waroom_on_call_escalation_policy" "policy1" {
  name = %[1]q

  time_slots = [
    { day_of_week = 1, start_time = "09:00", end_time = "18:00" },
  ]
}

resource "waroom_on_call_escalation_policy" "policy2" {
  name = %[2]q

  time_slots = [
    { day_of_week = 2, start_time = "09:00", end_time = "18:00" },
  ]
}

resource "waroom_on_call_trigger" "test" {
  name         = %[3]q
  service_name = %[4]q
%[5]s
}
`, policy1Name, policy2Name, name, serviceName, body)
}

// testAccCheckOnCallTriggerDestroy checks that every on-call trigger in the state is gone.
func testAccCheckOnCallTriggerDestroy(client *waroom.Client) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_on_call_trigger" {
				continue
			}
			_, err := client.OnCallTriggers.Get(context.Background(), waroom.GetOnCallTriggerParams{UUID: rs.Primary.Attributes["uuid"]})
			if err == nil {
				return fmt.Errorf("on-call trigger %s still exists", rs.Primary.Attributes["uuid"])
			}
			var apiErr *waroom.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
				return fmt.Errorf("checking on-call trigger %s: %w", rs.Primary.Attributes["uuid"], err)
			}
		}
		return nil
	}
}
