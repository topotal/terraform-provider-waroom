package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

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

func TestRunbookImportNamespace(t *testing.T) {
	valid := map[string]string{
		"/default":               "/default",
		"default":                "/default",
		"/api-server/recovery":   "/api-server/recovery",
		"api-server/recovery":    "/api-server/recovery",
		"/a/":                    "/a/",
		strings.Repeat("a", 254): "/" + strings.Repeat("a", 254),
	}
	for in, want := range valid {
		if got, ok := runbookImportNamespace(in); !ok || got != want {
			t.Errorf("runbookImportNamespace(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}

	for _, in := range []string{"", "/", "//a", "/with space", "/v1.2", "/" + strings.Repeat("a", 255)} {
		if got, ok := runbookImportNamespace(in); ok {
			t.Errorf("runbookImportNamespace(%q) = (%q, true), want an invalid ID", in, got)
		}
	}
}

func TestRunbookResourceNamespaceValidators(t *testing.T) {
	validate := func(raw string) bool {
		for _, v := range runbookResourceNamespaceValidators() {
			var resp validator.StringResponse
			v.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(raw)}, &resp)
			if resp.Diagnostics.HasError() {
				return false
			}
		}
		return true
	}

	for _, s := range []string{"/default", "/api-server/recovery", "/" + strings.Repeat("a", 254)} {
		if !validate(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range []string{"", "/", "default", "api-server/recovery", "//a", "/" + strings.Repeat("a", 255)} {
		if validate(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestAccRunbookResource(t *testing.T) {
	prefix := "/" + acctest.RandomWithPrefix("tf-acc")
	namespace := prefix + "/recovery"
	renamed := prefix + "/renamed/recovery"
	blob := "# Recovery\n\n1. Restart the server.\n"
	updatedBlob := "# Recovery\n\n1. Restart the server.\n2. Check the logs.\n"

	const name = "waroom_runbook.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRunbookDestroy(t),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccRunbookResourceConfig(namespace, blob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("namespace"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("updated_at"), knownvalue.NotNull()),
				},
			},
			// ImportState
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// ImportState without the leading slash
			{
				ResourceName:      name,
				ImportState:       true,
				ImportStateId:     namespace[1:],
				ImportStateVerify: true,
			},
			// A namespace without the leading slash is rejected at plan time
			{
				Config:      testAccRunbookResourceConfig(namespace[1:], blob),
				ExpectError: regexp.MustCompile("must start with `/`"),
			},
			// Update blob
			{
				Config: testAccRunbookResourceConfig(namespace, updatedBlob),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(updatedBlob)),
				},
			},
			// Rename in place and update blob
			{
				Config: testAccRunbookResourceConfig(renamed, blob),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(renamed)),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(renamed)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("namespace"), knownvalue.StringExact(renamed)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
				Check: testAccCheckRunbookGone(t, namespace),
			},
			// A runbook deleted outside Terraform is recreated
			{
				PreConfig: func() {
					err := testAccWaroomClient(t).Runbooks.Delete(context.Background(), waroom.DeleteRunbookParams{Namespace: runbookNamespaceParam(renamed)})
					if err != nil {
						t.Fatalf("delete runbook %s: %s", renamed, err)
					}
				},
				Config: testAccRunbookResourceConfig(renamed, blob),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(name, plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(renamed)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
			},
		},
	})
}

func testAccCheckRunbookDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "waroom_runbook" {
				continue
			}
			if err := testAccCheckRunbookGone(t, rs.Primary.Attributes["namespace"])(s); err != nil {
				return err
			}
		}
		return nil
	}
}

// testAccCheckRunbookGone checks that no runbook exists with the namespace.
func testAccCheckRunbookGone(t *testing.T, namespace string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client := testAccWaroomClient(t)
		_, err := client.Runbooks.Get(context.Background(), waroom.GetRunbookParams{Namespace: runbookNamespaceParam(namespace)})
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil
		}
		if err != nil {
			return fmt.Errorf("get runbook %s: %w", namespace, err)
		}
		return fmt.Errorf("runbook %s still exists", namespace)
	}
}

func testAccRunbookResourceConfig(namespace, blob string) string {
	return fmt.Sprintf(`
resource "waroom_runbook" "test" {
  namespace = %q
  blob      = %q
}
`, namespace, blob)
}
