package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"
)

// The service is created in the same config. The Waroom API can delete neither a service nor
// an architecture context, so both remain in Waroom; CheckDestroy verifies that destroy left the content as is.
func TestAccServiceArchitectureContextResource(t *testing.T) {
	serviceName := acctest.RandomWithPrefix("tf-acc")
	blob := "# Architecture\n\n- web: Rails\n- db: PostgreSQL\n"
	updatedBlob := "# Architecture\n\n- web: Rails\n- db: Aurora PostgreSQL\n- cache: Redis\n"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceArchitectureContextDestroy(t, updatedBlob),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccServiceArchitectureContextResourceConfig(serviceName, blob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("waroom_service_architecture_context.test", tfjsonpath.New("id"), knownvalue.StringExact(serviceName)),
					statecheck.ExpectKnownValue("waroom_service_architecture_context.test", tfjsonpath.New("service_name"), knownvalue.StringExact(serviceName)),
					statecheck.ExpectKnownValue("waroom_service_architecture_context.test", tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
			},
			// ImportState
			{
				ResourceName:      "waroom_service_architecture_context.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update
			{
				Config: testAccServiceArchitectureContextResourceConfig(serviceName, updatedBlob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("waroom_service_architecture_context.test", tfjsonpath.New("id"), knownvalue.StringExact(serviceName)),
					statecheck.ExpectKnownValue("waroom_service_architecture_context.test", tfjsonpath.New("blob"), knownvalue.StringExact(updatedBlob)),
				},
			},
		},
	})
}

// testAccCheckServiceArchitectureContextDestroy checks that destroy left the architecture context in Waroom unchanged.
func testAccCheckServiceArchitectureContextDestroy(t *testing.T, wantBlob string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccWaroomClient(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_service_architecture_context" {
				continue
			}
			got, err := client.Services.GetArchitectureContext(context.Background(), waroom.GetServiceArchitectureContextParams{
				Name: rs.Primary.Attributes["service_name"],
			})
			if err != nil {
				return fmt.Errorf("get architecture context of %q: %w", rs.Primary.Attributes["service_name"], err)
			}
			if got.Blob != wantBlob {
				return fmt.Errorf("architecture context of %q changed on destroy: got %q, want %q", rs.Primary.Attributes["service_name"], got.Blob, wantBlob)
			}
		}
		return nil
	}
}

func testAccServiceArchitectureContextResourceConfig(serviceName, blob string) string {
	return fmt.Sprintf(`
resource "waroom_service" "test" {
  name = %[1]q
}

resource "waroom_service_architecture_context" "test" {
  service_name = waroom_service.test.name
  blob         = %[2]q
}
`, serviceName, blob)
}
