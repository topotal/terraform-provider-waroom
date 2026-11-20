package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/topotal/waroom-client-go/waroom"
)

var (
	_ resource.Resource                = &ServiceResource{}
	_ resource.ResourceWithConfigure   = &ServiceResource{}
	_ resource.ResourceWithImportState = &ServiceResource{}
)

// NewServiceResource is a helper to simplify provider implementation.
func NewServiceResource() resource.Resource {
	return &ServiceResource{}
}

// ServiceResource is the resource implementation.
type ServiceResource struct {
	client *waroom.Client
}

// ServiceResourceModel describes the resource data model.
type ServiceResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// Metadata returns the resource type name.
func (r *ServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *ServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*waroom.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

// Schema defines the schema for the resource.
func (r *ServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Waroom service. A service is the unit that incidents, labels and other settings belong to.\n\n" +
			"~> **Note:** The Waroom API does not support deleting or renaming a service. " +
			"Destroying this resource only removes it from the Terraform state; the service remains in Waroom. " +
			"Changing `name` creates a new service and leaves the old one in Waroom.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Service ID. This is an alias for `name`, which identifies a service in Waroom.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the service. Letters, digits, underscores, hyphens, Hiragana, Katakana and Kanji are allowed. Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.Services.Create(ctx, waroom.CreateServiceReq{
		Name: plan.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("CreateService, got error: %s", err))
		return
	}

	var state ServiceResourceModel
	state.Id = types.StringValue(created.Name)
	state.Name = types.StringValue(created.Name)

	tflog.Info(ctx, "created service", map[string]interface{}{
		"id":   state.Id.ValueString(),
		"name": state.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
//
// There is no API to get a single service, so the service is looked up by name in the list.
func (r *ServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	services, err := r.client.Services.List(ctx, waroom.ListServicesParams{})
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
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update is never called because every attribute requires replacement.
func (r *ServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"The Waroom API does not support updating a service. Please report this issue to the provider developers.",
	)
}

// Delete removes the resource from the Terraform state.
//
// The Waroom API does not support deleting a service, so the service remains in Waroom.
func (r *ServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServiceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Service Not Deleted",
		fmt.Sprintf("The Waroom API does not support deleting a service. %q was removed from the Terraform state but remains in Waroom.", state.Name.ValueString()),
	)
}

// ImportState imports an existing service into Terraform state.
//
// The import ID is the name of the service.
//
// Example:
//
//	terraform import waroom_service.example my-service
func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
