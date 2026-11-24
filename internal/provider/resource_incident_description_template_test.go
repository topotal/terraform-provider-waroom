package provider

import (
	"context"
	"fmt"
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

// The template is organization-wide, so the test saves the current template and restores it afterwards.
func TestAccIncidentDescriptionTemplateResource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)
	testAccRestoreIncidentDescriptionTemplate(t)

	blob := "## Impact\n\n## Current status\n"
	updatedBlob := "## Impact\n\n## Current status\n\n## Next update\n"

	const name = "waroom_incident_description_template.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckIncidentDescriptionTemplateDestroy(t),
		Steps: []resource.TestStep{
			// A whitespace-only blob is rejected at plan time.
			// This step comes first because the post-test destroy uses the last step's config.
			{
				Config:      testAccIncidentDescriptionTemplateResourceConfig(" \n"),
				ExpectError: regexp.MustCompile(`must contain a non-whitespace character`),
			},
			// Create and Read
			{
				Config: testAccIncidentDescriptionTemplateResourceConfig(blob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(incidentDescriptionTemplateID)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
			},
			// ImportState
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// ImportState rejects any other ID
			{
				ResourceName:  name,
				ImportState:   true,
				ImportStateId: "other",
				ExpectError:   regexp.MustCompile(`Invalid Import ID`),
			},
			// Update blob in place
			{
				Config: testAccIncidentDescriptionTemplateResourceConfig(updatedBlob),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(updatedBlob)),
				},
			},
		},
	})
}

// testAccRestoreIncidentDescriptionTemplate puts the organization's current template back when the test ends.
func testAccRestoreIncidentDescriptionTemplate(t *testing.T) {
	t.Helper()
	client := testAccWaroomClient(t)

	original, err := client.IncidentDescriptionTemplate.Get(context.Background())
	if err != nil {
		t.Fatalf("get incident description template: %s", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if original.Blob != "" {
			_, err := client.IncidentDescriptionTemplate.Update(ctx, waroom.UpdateIncidentDescriptionTemplateReq{
				Blob: waroom.NewOptString(original.Blob),
			})
			if err != nil {
				t.Errorf("restore incident description template: %s", err)
			}
			return
		}

		// There was no template. Normally the destroy has already cleared it, so delete only
		// when one is left (e.g. the test failed midway).
		current, err := client.IncidentDescriptionTemplate.Get(ctx)
		if err != nil {
			t.Errorf("get incident description template: %s", err)
			return
		}
		if current.Blob == "" {
			return
		}
		if err := client.IncidentDescriptionTemplate.Delete(ctx); err != nil {
			t.Errorf("restore incident description template: %s", err)
		}
	})
}

func testAccCheckIncidentDescriptionTemplateDestroy(t *testing.T) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		tmpl, err := testAccWaroomClient(t).IncidentDescriptionTemplate.Get(context.Background())
		if err != nil {
			return fmt.Errorf("get incident description template: %w", err)
		}
		if tmpl.Blob != "" {
			return fmt.Errorf("incident description template still exists")
		}
		return nil
	}
}

func testAccIncidentDescriptionTemplateResourceConfig(blob string) string {
	return fmt.Sprintf(`
resource "waroom_incident_description_template" "test" {
  blob = %q
}
`, blob)
}
