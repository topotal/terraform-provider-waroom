package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/topotal/waroom-client-go/waroom"
)

var (
	_ resource.Resource              = &ServiceLabelResource{}
	_ resource.ResourceWithConfigure = &ServiceLabelResource{}
)

// NewServiceLabelResource is a helper to simplify provider implementation.
func NewServiceLabelResource() resource.Resource {
	return &ServiceLabelResource{}
}

// ServiceLabelResource is the resource implementation.
type ServiceLabelResource struct {
	client *waroom.WaroomClient
}

// ServiceLabelResourceModel describes the resource data model.
type ServiceLabelResourceModel struct {
	// example attribute
	// UUID        types.String `tfsdk:"uuid"`
}

// Metadata returns the resource type name.
func (r *ServiceLabelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_label"
}

func (r *ServiceLabelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*waroom.WaroomClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *WaroomClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

// Schema defines the schema for the resource.
func (r *ServiceLabelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// TODO: define attributes.
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *ServiceLabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// TODO: implement
}

// Read refreshes the Terraform state with the latest data.
func (r *ServiceLabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// TODO: implement
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *ServiceLabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// TODO: implement
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *ServiceLabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// TODO: implement
}
