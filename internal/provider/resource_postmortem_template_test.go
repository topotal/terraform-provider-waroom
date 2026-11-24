package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"
)

func TestAccPostmortemTemplateResource(t *testing.T) {
	title := acctest.RandomWithPrefix("tf-acc")
	updatedTitle := acctest.RandomWithPrefix("tf-acc")
	blob := "# " + title + "\n\n## Summary\n"
	updatedBlob := "# " + updatedTitle + "\n\n## Summary\n\n## Timeline\n"

	const name = "waroom_postmortem_template.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPostmortemTemplateDestroy(t),
		Steps: []resource.TestStep{
			// Create without a title and Read
			{
				Config: testAccPostmortemTemplateResourceConfig("", blob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("uuid"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("title"), knownvalue.Null()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("updated_at"), knownvalue.NotNull()),
				},
			},
			// Add a title
			{
				Config: testAccPostmortemTemplateResourceConfig(fmt.Sprintf("title = %q", title), blob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("title"), knownvalue.StringExact(title)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
			},
			// ImportState
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// An empty title is rejected at plan time
			{
				Config:      testAccPostmortemTemplateResourceConfig(`title = ""`, blob),
				ExpectError: regexp.MustCompile(`string length must be at least 1`),
			},
			// Update title and blob
			{
				Config: testAccPostmortemTemplateResourceConfig(fmt.Sprintf("title = %q", updatedTitle), updatedBlob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("title"), knownvalue.StringExact(updatedTitle)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(updatedBlob)),
				},
			},
			// Remove the title
			{
				Config: testAccPostmortemTemplateResourceConfig("", updatedBlob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("title"), knownvalue.Null()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(updatedBlob)),
				},
			},
		},
	})
}

func testAccCheckPostmortemTemplateDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccWaroomClient(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_postmortem_template" {
				continue
			}
			id, err := uuid.Parse(rs.Primary.Attributes["uuid"])
			if err != nil {
				return fmt.Errorf("parse uuid %q: %w", rs.Primary.Attributes["uuid"], err)
			}
			_, err = client.PostmortemTemplates.Get(context.Background(), waroom.GetPostmortemTemplateByUuidParams{UUID: id})
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				continue
			}
			if err != nil {
				return fmt.Errorf("get postmortem template %s: %w", id, err)
			}
			return fmt.Errorf("postmortem template %s still exists", id)
		}
		return nil
	}
}

func testAccPostmortemTemplateResourceConfig(title, blob string) string {
	return fmt.Sprintf(`
resource "waroom_postmortem_template" "test" {
  %s
  blob = %q
}
`, title, blob)
}
