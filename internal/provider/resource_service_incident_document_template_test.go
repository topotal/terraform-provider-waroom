package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"
)

// The test overwrites and clears the template of two services, so it uses services created by acceptance tests
// (named "tf-acc-*") rather than a real one such as E2E_WAROOM_SERVICE_NAME.
func TestAccServiceIncidentDocumentTemplateResource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	services := testAccServices(t, 2)
	serviceA, serviceB := services[0], services[1]
	blob := "# Incident\n\n## Impact\n"
	updatedBlob := "# Incident\n\n## Impact\n\n## Timeline\n"

	const name = "waroom_service_incident_document_template.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceIncidentDocumentTemplateDestroy(t),
		Steps: []resource.TestStep{
			// A whitespace-only blob is rejected at plan time.
			// This step comes first because the post-test destroy uses the last step's config.
			{
				Config:      testAccServiceIncidentDocumentTemplateResourceConfig(serviceA, serviceB, "a", " \n\u3000\u00a0"),
				ExpectError: regexp.MustCompile(`must contain a non-whitespace character`),
			},
			// Create and Read
			{
				Config: testAccServiceIncidentDocumentTemplateResourceConfig(serviceA, serviceB, "a", blob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(serviceA)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("service_name"), knownvalue.StringExact(serviceA)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
			},
			// ImportState
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update blob in place
			{
				Config: testAccServiceIncidentDocumentTemplateResourceConfig(serviceA, serviceB, "a", updatedBlob),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(updatedBlob)),
				},
			},
			// A template cleared outside Terraform is treated as absent and set again
			{
				PreConfig: func() {
					err := testAccWaroomClient(t).Services.DeleteIncidentDocumentTemplate(context.Background(),
						waroom.DeleteIncidentDocumentTemplateParams{Name: serviceA})
					if err != nil {
						t.Fatalf("delete incident document template of %s: %s", serviceA, err)
					}
				},
				Config: testAccServiceIncidentDocumentTemplateResourceConfig(serviceA, serviceB, "a", updatedBlob),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(name, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(updatedBlob)),
				},
			},
			// Changing the service replaces the resource
			{
				Config: testAccServiceIncidentDocumentTemplateResourceConfig(serviceA, serviceB, "b", updatedBlob),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(name, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(serviceB)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("service_name"), knownvalue.StringExact(serviceB)),
				},
			},
		},
	})
}

// testAccCheckServiceIncidentDocumentTemplateDestroy expects the template to be cleared (empty) or the service to be gone.
func testAccCheckServiceIncidentDocumentTemplateDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccWaroomClient(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_service_incident_document_template" {
				continue
			}
			serviceName := rs.Primary.Attributes["service_name"]
			tmpl, err := client.Services.GetIncidentDocumentTemplate(context.Background(), waroom.GetIncidentDocumentTemplateParams{Name: serviceName})
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				continue
			}
			if err != nil {
				return fmt.Errorf("get incident document template of %s: %w", serviceName, err)
			}
			if tmpl.Blob != "" {
				return fmt.Errorf("incident document template of %s still exists", serviceName)
			}
		}
		return nil
	}
}

// testAccServiceIncidentDocumentTemplateResourceConfig puts the template on service "a" or "b".
func testAccServiceIncidentDocumentTemplateResourceConfig(serviceA, serviceB, target, blob string) string {
	serviceName := serviceA
	if target == "b" {
		serviceName = serviceB
	}
	return fmt.Sprintf(`
resource "waroom_service_incident_document_template" "test" {
  service_name = %q
  blob         = %q
}
`, serviceName, blob)
}
