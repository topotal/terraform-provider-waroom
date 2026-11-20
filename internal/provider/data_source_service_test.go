package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccServiceDataSource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDataSourceConfig(name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.waroom_service.test", tfjsonpath.New("id"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("data.waroom_service.test", tfjsonpath.New("name"), knownvalue.StringExact(name)),
				},
			},
			{
				Config:      testAccServiceDataSourceNotFoundConfig(acctest.RandomWithPrefix("tf-acc-missing")),
				ExpectError: regexp.MustCompile(`Service Not Found`),
			},
		},
	})
}

func testAccServiceDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "waroom_service" "test" {
  name = %[1]q
}

data "waroom_service" "test" {
  name = waroom_service.test.name
}
`, name)
}

func testAccServiceDataSourceNotFoundConfig(name string) string {
	return fmt.Sprintf(`
data "waroom_service" "test" {
  name = %[1]q
}
`, name)
}
