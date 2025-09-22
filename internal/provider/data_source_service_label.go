/* JSON example response from the API
"labels": [
   {
       "uuid": "7dcf90f0-de30-4bd3-a797-3d53cef7ec7d",
       "name": "ないとぅー",
       "color": "E03708",
       "created_at": "2025-09-08T13:42:28.000Z",
       "updated_at": "2025-09-08T13:42:28.000Z"
   }
]
*/

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/topotal/waroom-client-go/waroom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ServiceLabelSource{}

func NewServiceLabelSource() datasource.DataSource {
	return &ServiceLabelSource{}
}

// ServiceLabelSource defines the data source implementation.
type ServiceLabelSource struct {
	client *waroom.WaroomClient
}

// ServiceLabelSourceModel describes the data source data model.
type ServiceLabelSourceModel struct {
	ServiceName types.String `tfsdk:"service_name"`
	UUID        types.String `tfsdk:"uuid"`
	Name        types.String `tfsdk:"name"`
	Color       types.String `tfsdk:"color"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (d *ServiceLabelSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_label"
}

func (d *ServiceLabelSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// TODO: write docs
		MarkdownDescription: "Waroom service label data source",

		Attributes: map[string]schema.Attribute{
			"service_name": schema.StringAttribute{
				MarkdownDescription: "The name of the service to which the label belongs.",
				Required:            true,
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "Label UUID.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Label name.",
				Required:            true,
			},
			"color": schema.StringAttribute{
				MarkdownDescription: "Label color in hex format(without#).",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "",
				Computed:            true,
			},
		},
	}
}

func (d *ServiceLabelSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*waroom.WaroomClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *ServiceLabelSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {

	// Read Terraform configuration data into the model
	var data ServiceLabelSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "mapping", map[string]interface{}{
		"uuid":  data.UUID.ValueString(),
		"name":  data.Name.ValueString(),
		"color": data.Color.ValueString(),
	})

	svc := waroom.NewLabelService(d.client)
	params := waroom.ListServiceLabelsParams{}

	if !data.ServiceName.IsNull() && !data.ServiceName.IsUnknown() {
		params.Name = data.ServiceName.ValueString()
	} else {
		resp.Diagnostics.AddError(
			"Service Name Required",
			"The service_name attribute is required.",
		)
		return
	}

	labels, err := svc.ListServiceLabels(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("ListServiceLabels, got error: %s", err))
		return
	}

	for _, label := range labels {
		// API response to model
		if label.UUID.String() == data.UUID.ValueString() || label.Name == data.Name.ValueString() {
			data.UUID = types.StringValue(label.UUID.String())
			data.Color = types.StringValue(label.Color.Value)
			data.CreatedAt = types.StringValue(label.CreatedAt.Value.String())
			data.Name = types.StringValue(label.Name)

			// data.UpdateAt = types.StringValue(label.UpdatedAt.Value.String())
		}
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
