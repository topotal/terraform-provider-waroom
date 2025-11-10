package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/topotal/terraform-provider-waroom/internal/utils"
	"github.com/topotal/waroom-client-go/waroom"
)

// Ensure WaroomProvider satisfies various provider interfaces.
var _ provider.Provider = &WaroomProvider{}
var _ provider.ProviderWithFunctions = &WaroomProvider{}
var _ provider.ProviderWithEphemeralResources = &WaroomProvider{}

// WaroomProvider defines the provider implementation.
type WaroomProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// WaroomProviderModel describes the provider data model.
type WaroomProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	ApiKey   types.String `tfsdk:"api_key"`
}

func (p *WaroomProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "waroom"
	resp.Version = p.version
}

func (p *WaroomProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Waroom API endpoint. This can also be set via the WAROOM_API_ENDPOINT environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "(Required unless validate is false) Waroom API key. This can also be set via the WAROOM_API_KEY environment variable.",
			},
		},
	}
}

func (p *WaroomProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data WaroomProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	endpoint := os.Getenv(utils.WaroomAPIEndPointEnvName)
	apiKey := os.Getenv(utils.WaroomAPIKeyEnvName)

	if !data.Endpoint.IsNull() {
		endpoint = data.Endpoint.ValueString()
	}

	if !data.ApiKey.IsNull() {
		apiKey = data.ApiKey.ValueString()
	}

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint)"),
			"Missing Waroom API endpoint",
			"The provider cannot create the Waroom API client as there is an unknown configuration value for the Waroom API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the WAROOM_API_ENDPOINT environment variable.",
		)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Waroom API key",
			"The provider cannot create the Waroom API client as there is an unknown configuration value for the Waroom API key. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the WAROOM_API_KEY environment variable.",
		)
	}

	// TODO: ここでAPI keyの疎通確認したほうが良いのかも？
	client, err := waroom.NewClient(ctx,
		waroom.WithBaseURL(endpoint),
		waroom.WithUserAgent("waroom-client-go 0.0.1/terraform-provider-waroom "+p.version),
		// waroom.WithRequestDump(),
		// waroom.WithResponseDump(),
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Waroom API Client",
			"An unexpected error occurred when creating the Waroom API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Waroom Client Error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *WaroomProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewServiceLabelResource,
	}
}

func (p *WaroomProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *WaroomProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewServiceLabelSource,
	}
}

func (p *WaroomProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &WaroomProvider{
			version: version,
		}
	}
}
