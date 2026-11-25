package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"
)

func TestParseServiceRunbookRuleImportID(t *testing.T) {
	const id = "123e4567-e89b-12d3-a456-426614174000"

	serviceName, got, err := parseServiceRunbookRuleImportID("my-service," + id)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if serviceName != "my-service" || got != id {
		t.Errorf("got (%q, %q), want (%q, %q)", serviceName, got, "my-service", id)
	}

	// Other UUID spellings are normalized to the canonical form the API returns.
	for _, in := range []string{strings.ToUpper(id), "{" + id + "}", "urn:uuid:" + id} {
		if _, got, err := parseServiceRunbookRuleImportID("my-service," + in); err != nil || got != id {
			t.Errorf("parseServiceRunbookRuleImportID(%q) = (%q, %v), want (%q, nil)", in, got, err, id)
		}
	}

	for _, in := range []string{"", id, "my-service", "my-service,", "," + id, "my-service,not-a-uuid", "a,b," + id} {
		if _, _, err := parseServiceRunbookRuleImportID(in); err == nil {
			t.Errorf("expected an error for %q", in)
		}
	}
}

func TestServiceRunbookRuleNamespaceValidators(t *testing.T) {
	valid := []string{
		"/default",
		"/api-server/recovery",
		"/A_z-0/9",
		"/" + strings.Repeat("a", 254),
	}
	invalid := []string{
		"",
		"/",
		"//a",
		"default",
		"api-server/recovery",
		"/with space",
		"/v1.2",
		"/日本語",
		"/" + strings.Repeat("a", 255),
	}

	validate := func(raw string) diag.Diagnostics {
		var diags diag.Diagnostics
		for _, v := range serviceRunbookRuleNamespaceValidators() {
			var resp validator.StringResponse
			v.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(raw)}, &resp)
			diags.Append(resp.Diagnostics...)
		}
		return diags
	}

	for _, s := range valid {
		if diags := validate(s); diags.HasError() {
			t.Errorf("expected %q to be valid, got: %v", s, diags)
		}
	}
	for _, s := range invalid {
		if !validate(s).HasError() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

// The service is one created by acceptance tests ("tf-acc-*"), since the Waroom API cannot delete a service.
// The runbooks are created through the API, as there is no waroom_runbook resource.
func TestAccServiceRunbookRuleResource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	serviceName := testAccServiceIncidentDocumentTemplateServices(t, 1)[0]
	prefix := "/" + acctest.RandomWithPrefix("tf-acc")
	namespaceA := prefix + "/a"
	namespaceB := prefix + "/b"
	testAccCreateRunbook(t, namespaceA, "# A\n")
	testAccCreateRunbook(t, namespaceB, "# B\n")

	const name = "waroom_service_runbook_rule.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceRunbookRuleDestroy(t),
		Steps: []resource.TestStep{
			// Values the API rejects are caught at plan time.
			// These steps come first because the post-test destroy uses the last step's config.
			{
				Config:      testAccServiceRunbookRuleResourceConfig(serviceName, "db", "title", -1, namespaceA),
				ExpectError: regexp.MustCompile(`must be at least 0`),
			},
			{
				Config:      testAccServiceRunbookRuleResourceConfig(serviceName, "db", "title", 10, namespaceA[1:]),
				ExpectError: regexp.MustCompile(`must start with`),
			},
			// The API rejects an invalid regular expression. The client does not surface the 422 message yet
			// (topotal/waroom-client-go#196), so only the status is checked.
			{
				Config:      testAccServiceRunbookRuleResourceConfig(serviceName, "(", "title", 10, namespaceA),
				ExpectError: regexp.MustCompile(`422`),
			},
			// Create and Read
			{
				Config: testAccServiceRunbookRuleResourceConfig(serviceName, "database.*down", "title", 10, namespaceA),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("uuid"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("service_name"), knownvalue.StringExact(serviceName)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("pattern"), knownvalue.StringExact("database.*down")),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("target"), knownvalue.StringExact("title")),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("priority"), knownvalue.Int64Exact(10)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("runbook_namespace"), knownvalue.StringExact(namespaceA)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("updated_at"), knownvalue.NotNull()),
				},
			},
			// ImportState
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[name]
					if !ok {
						return "", fmt.Errorf("resource %s not found", name)
					}
					return rs.Primary.Attributes["service_name"] + "," + rs.Primary.Attributes["uuid"], nil
				},
			},
			// Update every attribute in place
			{
				Config: testAccServiceRunbookRuleResourceConfig(serviceName, "(?i)latency", "description", 20, namespaceB),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("pattern"), knownvalue.StringExact("(?i)latency")),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("target"), knownvalue.StringExact("description")),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("priority"), knownvalue.Int64Exact(20)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("runbook_namespace"), knownvalue.StringExact(namespaceB)),
				},
			},
			// A rule deleted outside Terraform is recreated
			{
				PreConfig: func() {
					testAccDeleteServiceRunbookRules(t, serviceName, namespaceB)
				},
				Config: testAccServiceRunbookRuleResourceConfig(serviceName, "(?i)latency", "description", 20, namespaceB),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(name, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("runbook_namespace"), knownvalue.StringExact(namespaceB)),
				},
			},
		},
	})
}

// testAccDeleteServiceRunbookRules deletes the rules of a service that link to the given runbook.
func testAccDeleteServiceRunbookRules(t *testing.T, serviceName, namespace string) {
	t.Helper()
	client := testAccWaroomClient(t)
	rules, err := listAllServiceRunbookRules(context.Background(), client, serviceName)
	if err != nil {
		t.Fatalf("list runbook rules of %s: %s", serviceName, err)
	}
	for _, rule := range rules {
		if rule.RunbookNamespace != namespace {
			continue
		}
		err := client.RunbookRules.Delete(context.Background(), waroom.DeleteServiceRunbookRuleParams{Name: serviceName, UUID: rule.UUID})
		if err != nil {
			t.Fatalf("delete runbook rule %s of %s: %s", rule.UUID, serviceName, err)
		}
	}
}

// testAccCheckServiceRunbookRuleDestroy expects the rule to be absent from its service's list (or the service to be gone).
func testAccCheckServiceRunbookRuleDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccWaroomClient(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_service_runbook_rule" {
				continue
			}
			serviceName := rs.Primary.Attributes["service_name"]
			id, err := uuid.Parse(rs.Primary.Attributes["uuid"])
			if err != nil {
				return fmt.Errorf("parse uuid: %w", err)
			}
			rules, err := listAllServiceRunbookRules(context.Background(), client, serviceName)
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				continue
			}
			if err != nil {
				return fmt.Errorf("list runbook rules of %s: %w", serviceName, err)
			}
			for _, rule := range rules {
				if rule.UUID == id {
					return fmt.Errorf("runbook rule %s of %s still exists", id, serviceName)
				}
			}
		}
		return nil
	}
}

func testAccServiceRunbookRuleResourceConfig(serviceName, pattern, target string, priority int, runbookNamespace string) string {
	return fmt.Sprintf(`
resource "waroom_service_runbook_rule" "test" {
  service_name      = %q
  pattern           = %q
  target            = %q
  priority          = %d
  runbook_namespace = %q
}
`, serviceName, pattern, target, priority, runbookNamespace)
}
