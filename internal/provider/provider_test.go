package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

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
