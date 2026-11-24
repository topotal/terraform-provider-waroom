package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"
)

// The service is created in the same config so that the template of an existing service is never overwritten.
// The Waroom API cannot delete a service, so the service remains in Waroom.
func TestAccServiceIncidentDocumentTemplateResource(t *testing.T) {
	serviceName := acctest.RandomWithPrefix("tf-acc")
	blob := "# Incident\n\n## Impact\n"
	updatedBlob := "# Incident\n\n## Impact\n\n## Timeline\n"

	const name = "waroom_service_incident_document_template.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceIncidentDocumentTemplateDestroy(t),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccServiceIncidentDocumentTemplateResourceConfig(serviceName, blob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(serviceName)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("service_name"), knownvalue.StringExact(serviceName)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
			},
			// ImportState
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update blob
			{
				Config: testAccServiceIncidentDocumentTemplateResourceConfig(serviceName, updatedBlob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(updatedBlob)),
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

func testAccServiceIncidentDocumentTemplateResourceConfig(serviceName, blob string) string {
	return fmt.Sprintf(`
resource "waroom_service" "test" {
  name = %q
}

resource "waroom_service_incident_document_template" "test" {
  service_name = waroom_service.test.name
  blob         = %q
}
`, serviceName, blob)
}
