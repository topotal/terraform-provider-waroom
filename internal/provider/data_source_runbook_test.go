package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
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
	"github.com/topotal/waroom-client-go/waroom"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
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
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}
	testAccPreCheck(t)

	// There is no waroom_runbook resource, so the runbook is created through the API.
	namespace := "/" + acctest.RandomWithPrefix("tf-acc") + "/recovery"
	blob := "# Recovery\n\n1. Restart the server.\n"
	testAccCreateRunbook(t, namespace, blob)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunbookDataSourceConfig(namespace),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.waroom_runbook.test", tfjsonpath.New("id"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue("data.waroom_runbook.test", tfjsonpath.New("namespace"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue("data.waroom_runbook.test", tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
					statecheck.ExpectKnownValue("data.waroom_runbook.test", tfjsonpath.New("created_at"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.waroom_runbook.test", tfjsonpath.New("updated_at"), knownvalue.NotNull()),
				},
			},
			{
				// The leading slash is optional; id stays the canonical namespace.
				Config: testAccRunbookDataSourceConfig(namespace[1:]),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.waroom_runbook.test", tfjsonpath.New("id"), knownvalue.StringExact(namespace)),
					statecheck.ExpectKnownValue("data.waroom_runbook.test", tfjsonpath.New("namespace"), knownvalue.StringExact(namespace[1:])),
					statecheck.ExpectKnownValue("data.waroom_runbook.test", tfjsonpath.New("blob"), knownvalue.StringExact(blob)),
				},
			},
			{
				Config:      testAccRunbookDataSourceConfig("/" + acctest.RandomWithPrefix("tf-acc-missing")),
				ExpectError: regexp.MustCompile(`Runbook Not Found`),
			},
		},
	})
}

// testAccCreateRunbook creates a runbook through the API and deletes it when the test ends.
func testAccCreateRunbook(t *testing.T, namespace, blob string) {
	t.Helper()
	ctx := context.Background()

	client, err := waroom.NewClient(ctx,
		waroom.WithBaseURL(os.Getenv(utils.WaroomAPIEndPointEnvName)),
		waroom.WithAPIKey(os.Getenv(utils.WaroomAPIKeyEnvName)),
	)
	if err != nil {
		t.Fatalf("creating Waroom client: %s", err)
	}

	if _, err := client.Runbooks.Create(ctx, waroom.CreateRunbookReq{Namespace: namespace, Blob: blob}); err != nil {
		t.Fatalf("creating runbook %q: %s", namespace, err)
	}

	t.Cleanup(func() {
		err := client.Runbooks.Delete(context.Background(), waroom.DeleteRunbookParams{Namespace: runbookNamespaceParam(namespace)})
		var apiErr *waroom.APIError
		if err != nil && (!errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound) {
			t.Errorf("deleting runbook %q: %s", namespace, err)
		}
	})
}

func testAccRunbookDataSourceConfig(namespace string) string {
	return fmt.Sprintf(`
data "waroom_runbook" "test" {
  namespace = %[1]q
}
`, namespace)
}
