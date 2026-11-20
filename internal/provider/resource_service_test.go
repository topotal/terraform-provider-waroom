package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// The Waroom API cannot delete a service, so the services created here remain in Waroom
// and there is no CheckDestroy.
func TestAccServiceResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	renamed := acctest.RandomWithPrefix("tf-acc")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccServiceResourceConfig(name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("waroom_service.test", tfjsonpath.New("id"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("waroom_service.test", tfjsonpath.New("name"), knownvalue.StringExact(name)),
				},
			},
			// ImportState
			{
				ResourceName:      "waroom_service.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Changing the name replaces the service
			{
				Config: testAccServiceResourceConfig(renamed),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("waroom_service.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("waroom_service.test", tfjsonpath.New("id"), knownvalue.StringExact(renamed)),
					statecheck.ExpectKnownValue("waroom_service.test", tfjsonpath.New("name"), knownvalue.StringExact(renamed)),
				},
			},
		},
	})
}

func testAccServiceResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "waroom_service" "test" {
  name = %[1]q
}
`, name)
}
