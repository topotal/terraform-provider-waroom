package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/topotal/waroom-client-go/waroom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &ServiceDataSource{}
	_ datasource.DataSourceWithConfigure = &ServiceDataSource{}
)

// NewServiceDataSource is a helper to simplify provider implementation.
func NewServiceDataSource() datasource.DataSource {
	return &ServiceDataSource{}
}

// ServiceDataSource defines the data source implementation.
type ServiceDataSource struct {
	client *waroom.Client
}

// ServiceDataSourceModel describes the data source data model.
type ServiceDataSourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// Metadata returns the data source type name.
func (d *ServiceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (d *ServiceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*waroom.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = client
}

// Schema defines the schema for the data source.
func (d *ServiceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a Waroom service. Use this data source to check that a service exists " +
			"and to reference it from other resources, such as `waroom_service_label`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Service ID. This is an alias for `name`, which identifies a service in Waroom.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the service to look up.",
				Required:            true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
//
// There is no API to get a single service, so the service is looked up by name in the list.
func (d *ServiceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ServiceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	services, err := d.client.Services.List(ctx, waroom.ListServicesParams{})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("ListServices, got error: %s", err))
		return
	}

	var found bool
	for _, service := range services {
		if service.Name == data.Name.ValueString() {
			data.Id = types.StringValue(service.Name)
			data.Name = types.StringValue(service.Name)
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError(
			"Service Not Found",
			fmt.Sprintf("No Waroom service named %q was found.", data.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
