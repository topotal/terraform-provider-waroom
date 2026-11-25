package provider

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/topotal/waroom-client-go/waroom"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"waroom": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates that the environment variables required for acceptance testing are set.
func testAccPreCheck(t *testing.T) {
	for _, name := range []string{utils.WaroomAPIKeyEnvName, utils.WaroomAPIEndPointEnvName} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s must be set for acceptance tests", name)
		}
	}
}

// testAccServices returns n services created by acceptance tests ("tf-acc-*"),
// reusing existing ones and creating only the missing ones. The Waroom API cannot delete a service,
// so reusing them keeps the number of services from growing on every run.
func testAccServices(t *testing.T, n int) []string {
	t.Helper()
	ctx := context.Background()
	client := testAccWaroomClient(t)

	existing, err := client.Services.List(ctx, waroom.ListServicesParams{})
	if err != nil {
		t.Fatalf("list services: %s", err)
	}

	var names []string
	for _, svc := range existing {
		if len(names) < n && strings.HasPrefix(svc.Name, "tf-acc-") {
			names = append(names, svc.Name)
		}
	}
	for len(names) < n {
		svc, err := client.Services.Create(ctx, waroom.CreateServiceReq{Name: acctest.RandomWithPrefix("tf-acc")})
		if err != nil {
			t.Fatalf("create service: %s", err)
		}
		names = append(names, svc.Name)
	}
	return names
}
