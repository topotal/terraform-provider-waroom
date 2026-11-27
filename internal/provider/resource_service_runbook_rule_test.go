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
		for _, v := range runbookNamespaceWithSlashValidators() {
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
// The runbooks are created with waroom_runbook in the same config.
func TestAccServiceRunbookRuleResource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	serviceName := testAccServices(t, 1)[0]
	prefix := "/" + acctest.RandomWithPrefix("tf-acc")
	namespaceA := prefix + "/a"
	namespaceB := prefix + "/b"

	const name = "waroom_service_runbook_rule.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceRunbookRuleDestroy(t),
		Steps: []resource.TestStep{
			// Values the API rejects are caught at plan time.
			// These steps come first because the post-test destroy uses the last step's config.
			{
				Config:      testAccServiceRunbookRuleResourceConfig(serviceName, prefix, "db", "title", -1, "waroom_runbook.a.namespace"),
				ExpectError: regexp.MustCompile(`must be at least 0`),
			},
			{
				Config:      testAccServiceRunbookRuleResourceConfig(serviceName, prefix, "db", "title", 10, fmt.Sprintf("%q", namespaceA[1:])),
				ExpectError: regexp.MustCompile(`must start with`),
			},
			// The API rejects an invalid regular expression. The client does not surface the 422 message yet
			// (topotal/waroom-client-go#196), so only the status is checked.
			{
				Config:      testAccServiceRunbookRuleResourceConfig(serviceName, prefix, "(", "title", 10, "waroom_runbook.a.namespace"),
				ExpectError: regexp.MustCompile(`422`),
			},
			// Create and Read
			{
				Config: testAccServiceRunbookRuleResourceConfig(serviceName, prefix, "database.*down", "title", 10, "waroom_runbook.a.namespace"),
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
			// ImportState with an uppercase UUID: it is normalized to the API's form
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[name]
					if !ok {
						return "", fmt.Errorf("resource %s not found", name)
					}
					return rs.Primary.Attributes["service_name"] + "," + strings.ToUpper(rs.Primary.Attributes["uuid"]), nil
				},
			},
			// Update every attribute in place
			{
				Config: testAccServiceRunbookRuleResourceConfig(serviceName, prefix, "(?i)latency", "description", 20, "waroom_runbook.b.namespace"),
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
				Config: testAccServiceRunbookRuleResourceConfig(serviceName, prefix, "(?i)latency", "description", 20, "waroom_runbook.b.namespace"),
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

// testAccServiceRunbookRuleResourceConfig creates the runbooks <runbookPrefix>/a and <runbookPrefix>/b,
// and a rule whose runbook_namespace is the HCL expression runbookNamespaceExpr (e.g. waroom_runbook.a.namespace).
func testAccServiceRunbookRuleResourceConfig(serviceName, runbookPrefix, pattern, target string, priority int, runbookNamespaceExpr string) string {
	return fmt.Sprintf(`
resource "waroom_runbook" "a" {
  namespace = "%[1]s/a"
  blob      = "# A\n"
}

resource "waroom_runbook" "b" {
  namespace = "%[1]s/b"
  blob      = "# B\n"
}

resource "waroom_service_runbook_rule" "test" {
  service_name      = %[2]q
  pattern           = %[3]q
  target            = %[4]q
  priority          = %[5]d
  runbook_namespace = %[6]s
}
`, runbookPrefix, serviceName, pattern, target, priority, runbookNamespaceExpr)
}
