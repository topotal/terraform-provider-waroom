package provider

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
)

// There is no on-call trigger resource yet, so the trigger and the escalation policy it calls
// are created with the client before the test and deleted afterwards.
func TestAccOnCallTriggerDataSource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	client := testAccWaroomClient(t)
	serviceName := testAccOnCallTriggerService(t, client)
	policy := testAccCreateOnCallEscalationPolicy(t, client, acctest.RandomWithPrefix("tf-acc"))
	trigger := testAccCreateOnCallTrigger(t, client, acctest.RandomWithPrefix("tf-acc"), serviceName, policy.UUID)

	const ds = "data.waroom_on_call_trigger.test"
	allAttrs := []statecheck.StateCheck{
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("id"), knownvalue.StringExact(trigger.UUID)),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("uuid"), knownvalue.StringExact(trigger.UUID)),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("name"), knownvalue.StringExact(trigger.Name)),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("service_name"), knownvalue.StringExact(serviceName)),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("enabled"), knownvalue.Bool(false)),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("event_type"), knownvalue.StringExact("incident_created")),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("severity_filters"), knownvalue.ListExact([]knownvalue.Check{
			knownvalue.StringExact("critical"),
			knownvalue.StringExact("high"),
		})),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("source_filters"), knownvalue.ListExact([]knownvalue.Check{
			knownvalue.StringExact("integration"),
			knownvalue.StringExact("web"),
		})),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("retry_count"), knownvalue.Int64Exact(2)),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("escalation_policies"), knownvalue.ListExact([]knownvalue.Check{
			knownvalue.ObjectExact(map[string]knownvalue.Check{
				"uuid": knownvalue.StringExact(policy.UUID),
				"name": knownvalue.StringExact(policy.Name),
			}),
		})),
	}
	uuidOnly := []statecheck.StateCheck{
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("uuid"), knownvalue.StringExact(trigger.UUID)),
		statecheck.ExpectKnownValue(ds, tfjsonpath.New("service_name"), knownvalue.StringExact(serviceName)),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Invalid lookup keys are rejected before any API call.
			{
				Config:      testAccOnCallTriggerDataSourceConfig(fmt.Sprintf("service_name = %q", serviceName)),
				ExpectError: regexp.MustCompile(`Missing Attribute Configuration`),
			},
			{
				Config:      testAccOnCallTriggerDataSourceConfig(fmt.Sprintf("uuid = %q\n  service_name = %q", trigger.UUID, serviceName)),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			// Missing triggers.
			{
				Config:      testAccOnCallTriggerDataSourceConfig(fmt.Sprintf("name = %q", acctest.RandomWithPrefix("tf-acc-missing"))),
				ExpectError: regexp.MustCompile(`On-Call Trigger Not Found`),
			},
			{
				Config:      testAccOnCallTriggerDataSourceConfig(fmt.Sprintf("uuid = %q", "00000000-0000-4000-8000-000000000000")),
				ExpectError: regexp.MustCompile(`On-Call Trigger Not Found`),
			},
			// Look up by UUID (Get API).
			{
				Config:            testAccOnCallTriggerDataSourceConfig(fmt.Sprintf("uuid = %q", trigger.UUID)),
				ConfigStateChecks: allAttrs,
			},
			// Look up by name within a service (List API).
			{
				Config:            testAccOnCallTriggerDataSourceConfig(fmt.Sprintf("name = %q\n  service_name = %q", trigger.Name, serviceName)),
				ConfigStateChecks: allAttrs,
			},
			// Look up by name across services.
			{
				Config:            testAccOnCallTriggerDataSourceConfig(fmt.Sprintf("name = %q", trigger.Name)),
				ConfigStateChecks: uuidOnly,
			},
		},
	})
}

func testAccOnCallTriggerDataSourceConfig(args string) string {
	return fmt.Sprintf(`
data "waroom_on_call_trigger" "test" {
  %s
}
`, args)
}

// testAccWaroomClient builds a client from the same variables the provider reads.
func testAccWaroomClient(t *testing.T) *waroom.Client {
	t.Helper()
	client, err := waroom.NewClient(context.Background(),
		waroom.WithBaseURL(os.Getenv(utils.WaroomAPIEndPointEnvName)),
		waroom.WithAPIKey(os.Getenv(utils.WaroomAPIKeyEnvName)),
	)
	if err != nil {
		t.Fatalf("create Waroom client: %s", err)
	}
	return client
}

// testAccOnCallTriggerService returns E2E_WAROOM_SERVICE_NAME, or creates a service when it is unset.
// The Waroom API cannot delete a service, so a created service remains in Waroom.
func testAccOnCallTriggerService(t *testing.T, client *waroom.Client) string {
	t.Helper()
	if name := os.Getenv("E2E_WAROOM_SERVICE_NAME"); name != "" {
		return name
	}
	svc, err := client.Services.Create(context.Background(), waroom.CreateServiceReq{Name: acctest.RandomWithPrefix("tf-acc")})
	if err != nil {
		t.Fatalf("create service: %s", err)
	}
	return svc.Name
}

func testAccCreateOnCallEscalationPolicy(t *testing.T, client *waroom.Client, name string) *waroom.CreatedOnCallEscalationPolicy {
	t.Helper()
	policy, err := client.OnCallEscalationPolicies.Create(context.Background(), waroom.CreateOnCallEscalationPolicyReq{
		Name:      name,
		TimeSlots: []waroom.CreateOnCallEscalationPolicyTimeSlot{{DayOfWeek: 1, StartTime: "09:00", EndTime: "18:00"}},
	})
	if err != nil {
		t.Fatalf("create on-call escalation policy: %s", err)
	}
	t.Cleanup(func() {
		if err := client.OnCallEscalationPolicies.Delete(context.Background(), waroom.DeleteOnCallEscalationPolicyParams{UUID: policy.UUID}); err != nil {
			t.Errorf("delete on-call escalation policy %s: %s", policy.UUID, err)
		}
	})
	return policy
}

func testAccCreateOnCallTrigger(t *testing.T, client *waroom.Client, name, serviceName, policyUUID string) *waroom.CreatedOnCallTrigger {
	t.Helper()
	trigger, err := client.OnCallTriggers.Create(context.Background(), waroom.CreateOnCallTriggerReq{
		Name:                  name,
		ServiceName:           serviceName,
		EscalationPolicyUuids: []string{policyUUID},
		SeverityFilters:       []waroom.CreateOnCallTriggerSeverityFilter{"critical", "high"},
		SourceFilters:         []waroom.CreateOnCallTriggerSourceFilter{"integration", "web"},
		Enabled:               waroom.NewOptBool(false), // never call anyone from a test
		RetryCount:            waroom.NewOptInt(2),
	})
	if err != nil {
		t.Fatalf("create on-call trigger: %s", err)
	}
	t.Cleanup(func() {
		if err := client.OnCallTriggers.Delete(context.Background(), waroom.DeleteOnCallTriggerParams{UUID: trigger.UUID}); err != nil {
			t.Errorf("delete on-call trigger %s: %s", trigger.UUID, err)
		}
	})
	return trigger
}

func TestFilterOnCallTriggersByName(t *testing.T) {
	triggers := []onCallTrigger{
		{UUID: "1", Name: "critical", ServiceName: "a"},
		{UUID: "2", Name: "minor", ServiceName: "a"},
		{UUID: "3", Name: "critical", ServiceName: "b"},
	}

	tests := []struct {
		name string
		in   []onCallTrigger
		want []string
	}{
		{name: "critical", in: triggers, want: []string{"1", "3"}},
		{name: "minor", in: triggers, want: []string{"2"}},
		{name: "Critical", in: triggers, want: nil},
		{name: "critical", in: nil, want: nil},
	}
	for _, tt := range tests {
		var got []string
		for _, m := range filterOnCallTriggersByName(tt.in, tt.name) {
			got = append(got, m.UUID)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("filterOnCallTriggersByName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestOnCallTriggerDataSourceModel_fromAPI(t *testing.T) {
	ctx := context.Background()
	got := &waroom.OnCallTrigger{
		UUID:       "770e8400-e29b-41d4-a716-446655440001",
		Name:       "critical incidents",
		Enabled:    true,
		EventType:  "incident_created",
		RetryCount: 2,
	}
	got.Service.Name = "svc"
	got.SeverityFilters = append(got.SeverityFilters, "critical", "high")
	got.SourceFilters = append(got.SourceFilters, "integration")
	got.EscalationPolicies = slices.Grow(got.EscalationPolicies, 1)[:1] // the element type is internal to the client
	got.EscalationPolicies[0].UUID = "880e8400-e29b-41d4-a716-446655440001"
	got.EscalationPolicies[0].Name = "primary"

	var m OnCallTriggerDataSourceModel
	if diags := m.fromAPI(ctx, onCallTriggerFromGet(got)); diags.HasError() {
		t.Fatalf("fromAPI: %v", diags)
	}

	checks := map[string][2]any{
		"id":           {m.Id.ValueString(), "770e8400-e29b-41d4-a716-446655440001"},
		"uuid":         {m.UUID.ValueString(), "770e8400-e29b-41d4-a716-446655440001"},
		"name":         {m.Name.ValueString(), "critical incidents"},
		"service_name": {m.ServiceName.ValueString(), "svc"},
		"enabled":      {m.Enabled.ValueBool(), true},
		"event_type":   {m.EventType.ValueString(), "incident_created"},
		"retry_count":  {m.RetryCount.ValueInt64(), int64(2)},
	}
	for attr, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %v, want %v", attr, c[0], c[1])
		}
	}

	var severities, sources []string
	if diags := m.SeverityFilters.ElementsAs(ctx, &severities, false); diags.HasError() {
		t.Fatal(diags)
	}
	if diags := m.SourceFilters.ElementsAs(ctx, &sources, false); diags.HasError() {
		t.Fatal(diags)
	}
	if !reflect.DeepEqual(severities, []string{"critical", "high"}) {
		t.Errorf("severity_filters = %v", severities)
	}
	if !reflect.DeepEqual(sources, []string{"integration"}) {
		t.Errorf("source_filters = %v", sources)
	}

	policies := m.EscalationPolicies.Elements()
	if len(policies) != 1 {
		t.Fatalf("escalation_policies has %d elements, want 1", len(policies))
	}
	attrs := policies[0].(types.Object).Attributes()
	if !attrs["uuid"].Equal(types.StringValue("880e8400-e29b-41d4-a716-446655440001")) || !attrs["name"].Equal(types.StringValue("primary")) {
		t.Errorf("escalation_policies[0] = %v", attrs)
	}
}

func TestOnCallTriggerDataSourceModel_fromAPI_empty(t *testing.T) {
	var m OnCallTriggerDataSourceModel
	if diags := m.fromAPI(context.Background(), onCallTriggerFromListItem(&waroom.OnCallTriggerListItem{UUID: "x", Name: "n"})); diags.HasError() {
		t.Fatalf("fromAPI: %v", diags)
	}

	// Empty API arrays become empty lists, not null.
	for attr, l := range map[string]types.List{
		"severity_filters":    m.SeverityFilters,
		"source_filters":      m.SourceFilters,
		"escalation_policies": m.EscalationPolicies,
	} {
		if l.IsNull() || len(l.Elements()) != 0 {
			t.Errorf("%s = %v, want an empty list", attr, l)
		}
	}
}
