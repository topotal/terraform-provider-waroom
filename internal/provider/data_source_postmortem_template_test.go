package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/topotal/waroom-client-go/waroom"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
)

func TestAccPostmortemTemplateDataSource(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	// There is no waroom_postmortem_template resource, so create the template with the client directly.
	title := acctest.RandomWithPrefix("tf-acc")
	blob := "# " + title + "\n\n## Summary\n"
	id := testAccCreatePostmortemTemplate(t, title, blob)

	checks := func(name string) []statecheck.StateCheck {
		return []statecheck.StateCheck{
			statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(id)),
			statecheck.ExpectKnownValue(name, tfjsonpath.New("uuid"), knownvalue.StringExact(id)),
			statecheck.ExpectKnownValue(name, tfjsonpath.New("title"), knownvalue.StringExact(title)),
			statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
			statecheck.ExpectKnownValue(name, tfjsonpath.New("created_at"), knownvalue.NotNull()),
			statecheck.ExpectKnownValue(name, tfjsonpath.New("updated_at"), knownvalue.NotNull()),
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            testAccPostmortemTemplateDataSourceConfig(fmt.Sprintf("uuid = %q", id)),
				ConfigStateChecks: checks("data.waroom_postmortem_template.test"),
			},
			{
				Config:            testAccPostmortemTemplateDataSourceConfig(fmt.Sprintf("title = %q", title)),
				ConfigStateChecks: checks("data.waroom_postmortem_template.test"),
			},
			{
				Config:      testAccPostmortemTemplateDataSourceConfig(fmt.Sprintf("uuid = %q", uuid.NewString())),
				ExpectError: regexp.MustCompile(`Postmortem Template Not Found`),
			},
			{
				Config:      testAccPostmortemTemplateDataSourceConfig(fmt.Sprintf("title = %q", acctest.RandomWithPrefix("tf-acc-missing"))),
				ExpectError: regexp.MustCompile(`Postmortem Template Not Found`),
			},
		},
	})
}

// testAccCreatePostmortemTemplate creates a postmortem template through the API and deletes it when the test ends.
func testAccCreatePostmortemTemplate(t *testing.T, title, blob string) string {
	t.Helper()
	ctx := context.Background()

	client, err := waroom.NewClient(ctx,
		waroom.WithBaseURL(os.Getenv(utils.WaroomAPIEndPointEnvName)),
		waroom.WithAPIKey(os.Getenv(utils.WaroomAPIKeyEnvName)),
	)
	if err != nil {
		t.Fatalf("creating Waroom client: %s", err)
	}

	req := waroom.CreatePostmortemTemplateReq{Blob: blob}
	req.Title.SetTo(title)
	created, err := client.PostmortemTemplates.Create(ctx, req)
	if err != nil {
		t.Fatalf("creating postmortem template: %s", err)
	}

	t.Cleanup(func() {
		err := client.PostmortemTemplates.Delete(context.Background(), waroom.DeletePostmortemTemplateByUuidParams{UUID: created.UUID})
		if err != nil {
			t.Errorf("deleting postmortem template %s: %s", created.UUID, err)
		}
	})

	return created.UUID.String()
}

func testAccPostmortemTemplateDataSourceConfig(lookup string) string {
	return fmt.Sprintf(`
data "waroom_postmortem_template" "test" {
  %s
}
`, lookup)
}
