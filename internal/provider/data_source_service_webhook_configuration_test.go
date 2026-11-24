package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
)

func TestWebhookConfigurationToModel(t *testing.T) {
	var w waroom.WebhookConfiguration
	if err := json.Unmarshal([]byte(`{
		"uuid": "0b0f4c3e-6f0c-4d6c-9f3b-0f6f5a7b8c9d",
		"name": "my-webhook",
		"url": "https://example.com/hook",
		"state": "verified",
		"webhook_subscription_events": [
			{"uuid": "1c1f4c3e-6f0c-4d6c-9f3b-0f6f5a7b8c9d", "event_type": "incident_created", "event_version": "v1"}
		],
		"created_at": "2025-09-08T13:42:28.000Z",
		"updated_at": "2025-09-08T14:42:28.000Z"
	}`), &w); err != nil {
		t.Fatal(err)
	}

	m, diags := webhookConfigurationToModel("svc", &w)
	if diags.HasError() {
		t.Fatal(diags)
	}

	assertEqual(t, "0b0f4c3e-6f0c-4d6c-9f3b-0f6f5a7b8c9d", m.Id.ValueString())
	assertEqual(t, "0b0f4c3e-6f0c-4d6c-9f3b-0f6f5a7b8c9d", m.UUID.ValueString())
	assertEqual(t, "svc", m.ServiceName.ValueString())
	assertEqual(t, "my-webhook", m.Name.ValueString())
	assertEqual(t, "https://example.com/hook", m.URL.ValueString())
	assertEqual(t, "verified", m.State.ValueString())
	assertEqual(t, "2025-09-08T13:42:28Z", m.CreatedAt.ValueString())
	assertEqual(t, "2025-09-08T14:42:28Z", m.UpdatedAt.ValueString())

	events := m.WebhookSubscriptionEvents.Elements()
	if len(events) != 1 {
		t.Fatalf("events: got %d, want 1", len(events))
	}
	attrs := events[0].(types.Object).Attributes()
	assertEqual(t, types.StringValue("1c1f4c3e-6f0c-4d6c-9f3b-0f6f5a7b8c9d"), attrs["uuid"])
	assertEqual(t, types.StringValue("incident_created"), attrs["event_type"])
	assertEqual(t, types.StringValue("v1"), attrs["event_version"])
}

// There is no API to create a webhook configuration, so the acceptance test reads one that already
// exists in the service given by E2E_WAROOM_SERVICE_NAME, found through the client.
func TestAccServiceWebhookConfigurationDataSource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	serviceName := os.Getenv("E2E_WAROOM_SERVICE_NAME")
	if serviceName == "" {
		t.Skip("E2E_WAROOM_SERVICE_NAME must be set to a Waroom service that has at least one webhook configuration")
	}
	w := testAccFindWebhookConfiguration(t, serviceName)

	expect := func(attr string, v knownvalue.Check) statecheck.StateCheck {
		return statecheck.ExpectKnownValue("data.waroom_service_webhook_configuration.test", tfjsonpath.New(attr), v)
	}
	checks := []statecheck.StateCheck{
		expect("id", knownvalue.StringExact(w.UUID.String())),
		expect("uuid", knownvalue.StringExact(w.UUID.String())),
		expect("service_name", knownvalue.StringExact(serviceName)),
		expect("name", knownvalue.StringExact(w.Name)),
		expect("url", knownvalue.StringExact(w.URL.String())),
		expect("state", knownvalue.StringExact(string(w.State))),
		expect("webhook_subscription_events", knownvalue.ListSizeExact(len(w.WebhookSubscriptionEvents))),
		expect("created_at", knownvalue.NotNull()),
		expect("updated_at", knownvalue.NotNull()),
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            testAccServiceWebhookConfigurationDataSourceConfig(serviceName, "uuid", w.UUID.String()),
				ConfigStateChecks: checks,
			},
			{
				Config:            testAccServiceWebhookConfigurationDataSourceConfig(serviceName, "name", w.Name),
				ConfigStateChecks: checks,
			},
			{
				Config:      testAccServiceWebhookConfigurationDataSourceConfig(serviceName, "name", acctest.RandomWithPrefix("tf-acc-missing")),
				ExpectError: regexp.MustCompile(`Webhook Configuration Not Found`),
			},
			{
				Config:      testAccServiceWebhookConfigurationDataSourceConfig(serviceName, "uuid", uuid.NewString()),
				ExpectError: regexp.MustCompile(`Webhook Configuration Not Found`),
			},
		},
	})
}

// testAccFindWebhookConfiguration returns a webhook configuration of the service whose name is
// unique in it (so that it can also be looked up by name), skipping the test when there is none.
func testAccFindWebhookConfiguration(t *testing.T, serviceName string) *waroom.WebhookConfiguration {
	t.Helper()
	ctx := context.Background()

	client, err := waroom.NewClient(ctx,
		waroom.WithBaseURL(os.Getenv(utils.WaroomAPIEndPointEnvName)),
		waroom.WithAPIKey(os.Getenv(utils.WaroomAPIKeyEnvName)),
	)
	if err != nil {
		t.Fatal(err)
	}

	items, err := client.WebhookConfigurations.List(ctx, waroom.ListServiceWebhookConfigurationsParams{
		Name:    serviceName,
		PerPage: waroom.NewOptInt(webhookConfigurationsPerPage),
	})
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	for _, item := range items {
		counts[item.Name]++
	}
	for _, item := range items {
		if counts[item.Name] != 1 {
			continue
		}
		w, err := client.WebhookConfigurations.Get(ctx, waroom.GetServiceWebhookConfigurationParams{Name: serviceName, UUID: item.UUID})
		if err != nil {
			t.Fatal(err)
		}
		return w
	}

	t.Skipf("the Waroom service %q (E2E_WAROOM_SERVICE_NAME) has no webhook configuration with a unique name; create one in Waroom (there is no API for it)", serviceName)
	return nil
}

func testAccServiceWebhookConfigurationDataSourceConfig(serviceName, key, value string) string {
	return fmt.Sprintf(`
data "waroom_service_webhook_configuration" "test" {
  service_name = %[1]q
  %[2]s = %[3]q
}
`, serviceName, key, value)
}

func assertEqual(t *testing.T, want, got any) {
	t.Helper()
	if want != got {
		t.Errorf("got %v, want %v", got, want)
	}
}
