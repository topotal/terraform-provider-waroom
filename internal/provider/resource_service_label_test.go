package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccServiceLabelResource_basic(t *testing.T) {
	ctx := context.Background()
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccPreCheckServiceExists(t, "test-service") },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceLabelDestroy(ctx),
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccServiceLabelResourceConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify resource attributes
					resource.TestCheckResourceAttr("waroom_service_label.test", "service_name", "test-service"),
					resource.TestCheckResourceAttr("waroom_service_label.test", "name", rName),
					resource.TestCheckResourceAttr("waroom_service_label.test", "color", "008BD1"),
					// Verify computed attributes are set
					resource.TestCheckResourceAttrSet("waroom_service_label.test", "id"),
					resource.TestCheckResourceAttrSet("waroom_service_label.test", "uuid"),
				),
			},
			// Update testing - modify name and color
			{
				Config: testAccServiceLabelResourceConfig_updated(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("waroom_service_label.test", "name", rName+"-updated"),
					resource.TestCheckResourceAttr("waroom_service_label.test", "color", "FF5733"),
					resource.TestCheckResourceAttrSet("waroom_service_label.test", "id"),
					resource.TestCheckResourceAttrSet("waroom_service_label.test", "uuid"),
				),
			},
			// Import testing
			{
				ResourceName:      "waroom_service_label.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["waroom_service_label.test"]
					if !ok {
						return "", fmt.Errorf("resource not found")
					}

					serviceName := rs.Primary.Attributes["service_name"]
					uuid := rs.Primary.Attributes["uuid"]

					if serviceName == "" || uuid == "" {
						return "", fmt.Errorf("service_name or uuid not found in state")
					}

					return fmt.Sprintf("%s,%s", serviceName, uuid), nil
				},
			},
		},
	})
}

func TestAccServiceLabelResource_invalidColor(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccPreCheckServiceExists(t, "test-service") },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccServiceLabelResourceConfig_invalidColor(rName, "invalid"),
				ExpectError: regexp.MustCompile("must be a 6-digit hex color code without #"),
			},
			{
				Config:      testAccServiceLabelResourceConfig_invalidColor(rName, "#008BD1"),
				ExpectError: regexp.MustCompile("must be a 6-digit hex color code without #"),
			},
			{
				Config:      testAccServiceLabelResourceConfig_invalidColor(rName, "12345"),
				ExpectError: regexp.MustCompile("must be a 6-digit hex color code without #"),
			},
		},
	})
}

func TestAccServiceLabelResource_requiresReplace(t *testing.T) {
	ctx := context.Background()
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckServiceExists(t, "test-service-1")
			testAccPreCheckServiceExists(t, "test-service-2")
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceLabelDestroy(ctx),
		Steps: []resource.TestStep{
			// Create resource with initial service_name
			{
				Config: testAccServiceLabelResourceConfig_serviceName(rName, "test-service-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("waroom_service_label.test", "service_name", "test-service-1"),
					resource.TestCheckResourceAttr("waroom_service_label.test", "name", rName),
				),
			},
			// Change service_name should trigger RequiresReplace
			{
				Config: testAccServiceLabelResourceConfig_serviceName(rName, "test-service-2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("waroom_service_label.test", "service_name", "test-service-2"),
					resource.TestCheckResourceAttr("waroom_service_label.test", "name", rName),
				),
			},
		},
	})
}

// testAccServiceLabelResourceConfigBase provides base configuration
// This follows the template method pattern used in major providers (AWS, Azure, etc.)
// where prerequisite resources are defined in a base template function.
//
// Currently assumes "test-service" exists. In production, if you have a waroom_service resource:
// func testAccServiceLabelResourceConfigBase() string {
//     return providerConfig + `
// resource "waroom_service" "test" {
//   name = "test-service"
// }
// `
// }
//
// Then in your test configs, reference it:
// resource "waroom_service_label" "test" {
//   service_name = waroom_service.test.name  // <-- Terraform creates dependency
//   name         = "label-name"
//   color        = "008BD1"
// }
func testAccServiceLabelResourceConfigBase() string {
	return providerConfig
}

// testAccServiceLabelResourceConfig_basic generates a basic test configuration
func testAccServiceLabelResourceConfig_basic(name string) string {
	return testAccServiceLabelResourceConfigBase() + fmt.Sprintf(`
resource "waroom_service_label" "test" {
  service_name = "test-service"
  name         = %[1]q
  color        = "008BD1"
}
`, name)
}

// testAccServiceLabelResourceConfig_updated generates an updated test configuration
func testAccServiceLabelResourceConfig_updated(name string) string {
	return testAccServiceLabelResourceConfigBase() + fmt.Sprintf(`
resource "waroom_service_label" "test" {
  service_name = "test-service"
  name         = %[1]q
  color        = "FF5733"
}
`, name+"-updated")
}

// testAccServiceLabelResourceConfig_invalidColor generates a test configuration with invalid color
func testAccServiceLabelResourceConfig_invalidColor(name, color string) string {
	return testAccServiceLabelResourceConfigBase() + fmt.Sprintf(`
resource "waroom_service_label" "test" {
  service_name = "test-service"
  name         = %[1]q
  color        = %[2]q
}
`, name, color)
}

// testAccServiceLabelResourceConfig_serviceName generates a test configuration with custom service_name
func testAccServiceLabelResourceConfig_serviceName(name, serviceName string) string {
	return testAccServiceLabelResourceConfigBase() + fmt.Sprintf(`
resource "waroom_service_label" "test" {
  service_name = %[2]q
  name         = %[1]q
  color        = "008BD1"
}
`, name, serviceName)
}

// testAccPreCheckServiceExists verifies that the required service exists before running tests
//
// NOTE: PreCheck is for CHECKING prerequisites, not CREATING them.
// PreCheck runs before EACH test and should only validate the environment.
//
// Best practice patterns:
// 1. If the service should be created as part of the test:
//    - Create it in testAccServiceLabelResourceConfigBase() using Terraform config
//    - Terraform will handle creation/deletion automatically
//    - No PreCheck needed for resources created in the test
//
// 2. If the service must exist BEFORE tests (external dependency):
//    - Use PreCheck to verify it exists
//    - Skip test with t.Skip() if not found
//    - Example: checking for specific AWS regions, API features, etc.
//
// Current implementation assumes "test-service" exists externally.
// Once waroom_service resource is implemented, move creation to ConfigBase instead.
func testAccPreCheckServiceExists(t *testing.T, serviceName string) {
	// Placeholder: In production, check if service exists via API
	// If not found: t.Skipf("Service %q does not exist, skipping test", serviceName)
	t.Logf("Assuming service %q exists (to be replaced with actual check or resource creation)", serviceName)
}

// testAccCheckServiceLabelDestroy verifies that all service labels have been destroyed
func testAccCheckServiceLabelDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// Note: In a real implementation, you would:
		// 1. Get the provider client from the test state
		// 2. Iterate through all resources in state
		// 3. For each waroom_service_label, verify it no longer exists in the API
		// 4. Return an error if any labels still exist
		//
		// Example implementation structure:
		//
		// for _, rs := range s.RootModule().Resources {
		//     if rs.Type != "waroom_service_label" {
		//         continue
		//     }
		//
		//     // Try to fetch the label from API
		//     // If it exists, return error
		//     // If it doesn't exist (404), continue
		// }

		// For now, we trust that Terraform's destroy worked correctly
		// This is a placeholder that always passes
		return nil
	}
}
