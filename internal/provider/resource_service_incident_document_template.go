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
	_ resource.Resource                = &ServiceIncidentDocumentTemplateResource{}
	_ resource.ResourceWithConfigure   = &ServiceIncidentDocumentTemplateResource{}
	_ resource.ResourceWithImportState = &ServiceIncidentDocumentTemplateResource{}
)

// NewServiceIncidentDocumentTemplateResource is a helper to simplify provider implementation.
func NewServiceIncidentDocumentTemplateResource() resource.Resource {
	return &ServiceIncidentDocumentTemplateResource{}
}

// ServiceIncidentDocumentTemplateResource is the resource implementation.
type ServiceIncidentDocumentTemplateResource struct {
	client *waroom.Client
}

// ServiceIncidentDocumentTemplateResourceModel describes the resource data model.
type ServiceIncidentDocumentTemplateResourceModel struct {
	Id          types.String `tfsdk:"id"`
	ServiceName types.String `tfsdk:"service_name"`
	Blob        types.String `tfsdk:"blob"`
}

// Metadata returns the resource type name.
func (r *ServiceIncidentDocumentTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_incident_document_template"
}

func (r *ServiceIncidentDocumentTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *ServiceIncidentDocumentTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the incident document template of a Waroom service. " +
			"The template provides the initial content of the incident documents created in the service.\n\n" +
			"~> **Note:** A service has exactly one incident document template. " +
			"Creating this resource overwrites any template already set on the service, " +
			"and destroying it clears the template.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Resource ID. This is an alias for `service_name`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_name": schema.StringAttribute{
				MarkdownDescription: "The name of the Waroom service the template belongs to. Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"blob": schema.StringAttribute{
				MarkdownDescription: "The content of the incident document template, in Markdown. Must not be empty; destroy the resource to clear the template.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

// Create sets the template of the service and sets the initial Terraform state.
//
// The API has no create operation; the template is upserted with UpdateIncidentDocumentTemplate.
func (r *ServiceIncidentDocumentTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceIncidentDocumentTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.put(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateIncidentDocumentTemplate, got error: %s", err))
		return
	}

	tflog.Info(ctx, "created service incident document template", map[string]interface{}{
		"id":           state.Id.ValueString(),
		"service_name": state.ServiceName.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
//
// An empty template is treated as absent, as is a missing service.
func (r *ServiceIncidentDocumentTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceIncidentDocumentTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tmpl, err := r.client.Services.GetIncidentDocumentTemplate(ctx, waroom.GetIncidentDocumentTemplateParams{
		Name: data.ServiceName.ValueString(),
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetIncidentDocumentTemplate, got error: %s", err))
		return
	}

	if tmpl.Blob == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Id = data.ServiceName
	data.Blob = types.StringValue(tmpl.Blob)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the template and sets the updated Terraform state on success.
func (r *ServiceIncidentDocumentTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceIncidentDocumentTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.put(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateIncidentDocumentTemplate, got error: %s", err))
		return
	}

	tflog.Info(ctx, "updated service incident document template", map[string]interface{}{
		"id":           state.Id.ValueString(),
		"service_name": state.ServiceName.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete clears the template of the service and removes the Terraform state on success.
func (r *ServiceIncidentDocumentTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServiceIncidentDocumentTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Services.DeleteIncidentDocumentTemplate(ctx, waroom.DeleteIncidentDocumentTemplateParams{
		Name: state.ServiceName.ValueString(),
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("DeleteIncidentDocumentTemplate, got error: %s", err))
		return
	}
}

// ImportState imports the incident document template of an existing service into Terraform state.
//
// The import ID is the name of the service.
//
// Example:
//
//	terraform import waroom_service_incident_document_template.example my-service
func (r *ServiceIncidentDocumentTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("service_name"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// put upserts the template from the plan and returns the resulting state.
func (r *ServiceIncidentDocumentTemplateResource) put(ctx context.Context, plan ServiceIncidentDocumentTemplateResourceModel) (ServiceIncidentDocumentTemplateResourceModel, error) {
	updated, err := r.client.Services.UpdateIncidentDocumentTemplate(ctx,
		waroom.UpdateIncidentDocumentTemplateReq{Blob: waroom.NewOptString(plan.Blob.ValueString())},
		waroom.UpdateIncidentDocumentTemplateParams{Name: plan.ServiceName.ValueString()},
	)
	if err != nil {
		return ServiceIncidentDocumentTemplateResourceModel{}, err
	}

	return ServiceIncidentDocumentTemplateResourceModel{
		Id:          plan.ServiceName,
		ServiceName: plan.ServiceName,
		Blob:        types.StringValue(updated.Blob),
	}, nil
}
