package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestRunbookNamespaceParam(t *testing.T) {
	cases := map[string]string{
		"/default":             "default",
		"default":              "default",
		"/api-server/recovery": "api-server/recovery",
		"api-server/recovery":  "api-server/recovery",
	}
	for in, want := range cases {
		if got := runbookNamespaceParam(in); got != want {
			t.Errorf("runbookNamespaceParam(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunbookNamespaceValidators(t *testing.T) {
	valid := []string{
		"/default",
		"default",
		"/api-server/recovery",
		"api-server/recovery",
		"/A_z-0/9",
		"/a/",
		"/" + strings.Repeat("a", 254),
		strings.Repeat("a", 254),
	}
	invalid := []string{
		"",
		"/",
		"//a",
		"/with space",
		"/v1.2",
		"/日本語",
		"/" + strings.Repeat("a", 255),
		strings.Repeat("a", 255),
	}

	validate := func(raw string) diag.Diagnostics {
		var diags diag.Diagnostics
		for _, v := range runbookNamespaceValidators() {
			var resp validator.StringResponse
			v.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(raw)}, &resp)
			diags.Append(resp.Diagnostics...)
		}
		return diags
	}

	for _, s := range valid {
		if diags := validate(s); diags.HasError() {
			t.Errorf("expected %q to be valid, got: %v", s, diags)
		}
	}
	for _, s := range invalid {
		if !validate(s).HasError() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestAccRunbookDataSource(t *testing.T) {
	namespace := "/" + acctest.RandomWithPrefix("tf-acc") + "/recovery"
	blob := "# Recovery\n\n1. Restart the server.\n"

	const name = "data.waroom_runbook.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunbookDataSourceConfig(namespace, blob, "waroom_runbook.test.namespace"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("namespace"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("updated_at"), knownvalue.NotNull()),
				},
			},
			{
				// The leading slash is optional; id stays the canonical namespace.
				Config: testAccRunbookDataSourceConfig(namespace, blob, `trimprefix(waroom_runbook.test.namespace, "/")`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(name, tfjsonpath.New("id"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("namespace"), knownvalue.StringExact(namespace[1:])),
					statecheck.ExpectKnownValue(name, tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
			},
			{
				Config:      testAccRunbookDataSourceConfig(namespace, blob, fmt.Sprintf("%q", "/"+acctest.RandomWithPrefix("tf-acc-missing"))),
				ExpectError: regexp.MustCompile(`Runbook Not Found`),
			},
		},
	})
}

// testAccRunbookDataSourceConfig creates a runbook and looks up the namespace given by the HCL expression namespaceExpr.
func testAccRunbookDataSourceConfig(namespace, blob, namespaceExpr string) string {
	return fmt.Sprintf(`
resource "waroom_runbook" "test" {
  namespace = %q
  blob      = %q
}

data "waroom_runbook" "test" {
  namespace = %s
}
`, namespace, blob, namespaceExpr)
}
