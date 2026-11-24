package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

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
	_ resource.Resource                = &ServiceArchitectureContextResource{}
	_ resource.ResourceWithConfigure   = &ServiceArchitectureContextResource{}
	_ resource.ResourceWithImportState = &ServiceArchitectureContextResource{}
)

// NewServiceArchitectureContextResource is a helper to simplify provider implementation.
func NewServiceArchitectureContextResource() resource.Resource {
	return &ServiceArchitectureContextResource{}
}

// ServiceArchitectureContextResource is the resource implementation.
type ServiceArchitectureContextResource struct {
	client *waroom.Client
}

// ServiceArchitectureContextResourceModel describes the resource data model.
type ServiceArchitectureContextResourceModel struct {
	Id          types.String `tfsdk:"id"`
	ServiceName types.String `tfsdk:"service_name"`
	Blob        types.String `tfsdk:"blob"`
}

// Metadata returns the resource type name.
func (r *ServiceArchitectureContextResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_architecture_context"
}

func (r *ServiceArchitectureContextResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *ServiceArchitectureContextResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Waroom service architecture context. The architecture context is a Markdown document " +
			"describing the architecture of a service. Each service has exactly one architecture context.\n\n" +
			"~> **Note:** The Waroom API has no endpoint to delete an architecture context. " +
			"Creating this resource overwrites any existing architecture context of the service, " +
			"and destroying it only removes it from the Terraform state; the content remains in Waroom.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Architecture context ID. This is an alias for `service_name`, since there is one architecture context per service.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_name": schema.StringAttribute{
				MarkdownDescription: "The name of the Waroom service the architecture context belongs to. Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"blob": schema.StringAttribute{
				MarkdownDescription: "The content of the architecture context, in Markdown. Must not be empty.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

// put writes the architecture context of the service and returns the resulting state.
// The API is an upsert, so it is used by both Create and Update.
func (r *ServiceArchitectureContextResource) put(ctx context.Context, serviceName, blob string) (*ServiceArchitectureContextResourceModel, error) {
	updated, err := r.client.Services.UpdateArchitectureContext(ctx,
		waroom.UpdateArchitectureContextReq{Blob: blob},
		waroom.UpdateServiceArchitectureContextParams{Name: serviceName},
	)
	if err != nil {
		return nil, err
	}

	return &ServiceArchitectureContextResourceModel{
		Id:          types.StringValue(updated.Service.Name),
		ServiceName: types.StringValue(updated.Service.Name),
		Blob:        types.StringValue(updated.Blob),
	}, nil
}

// Create sets the architecture context of the service, overwriting any existing content.
func (r *ServiceArchitectureContextResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceArchitectureContextResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.put(ctx, plan.ServiceName.ValueString(), plan.Blob.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateServiceArchitectureContext, got error: %s", err))
		return
	}

	tflog.Info(ctx, "created service architecture context", map[string]interface{}{
		"id":           state.Id.ValueString(),
		"service_name": state.ServiceName.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *ServiceArchitectureContextResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceArchitectureContextResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.client.Services.GetArchitectureContext(ctx, waroom.GetServiceArchitectureContextParams{
		Name: data.ServiceName.ValueString(),
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetServiceArchitectureContext, got error: %s", err))
		return
	}

	data.Id = types.StringValue(got.Service.Name)
	data.ServiceName = types.StringValue(got.Service.Name)
	data.Blob = types.StringValue(got.Blob)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *ServiceArchitectureContextResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceArchitectureContextResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.put(ctx, plan.ServiceName.ValueString(), plan.Blob.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateServiceArchitectureContext, got error: %s", err))
		return
	}

	tflog.Info(ctx, "updated service architecture context", map[string]interface{}{
		"id":           state.Id.ValueString(),
		"service_name": state.ServiceName.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete removes the resource from the Terraform state.
//
// The Waroom API has no endpoint to delete an architecture context, and it rejects an empty
// content, so the content cannot be cleared either. It remains in Waroom.
func (r *ServiceArchitectureContextResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServiceArchitectureContextResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Service Architecture Context Not Deleted",
		fmt.Sprintf("The Waroom API does not support deleting an architecture context. The architecture context of %q was removed from the Terraform state but remains in Waroom.", state.ServiceName.ValueString()),
	)
}

// ImportState imports the architecture context of an existing service into Terraform state.
//
// The import ID is the name of the service, since there is one architecture context per service.
//
// Example:
//
//	terraform import waroom_service_architecture_context.example my-service
func (r *ServiceArchitectureContextResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("service_name"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
