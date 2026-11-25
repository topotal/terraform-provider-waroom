package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/topotal/waroom-client-go/waroom"

	waroomvalidator "github.com/topotal/terraform-provider-waroom/internal/validator"
)

var (
	_ resource.Resource                = &IncidentDescriptionTemplateResource{}
	_ resource.ResourceWithConfigure   = &IncidentDescriptionTemplateResource{}
	_ resource.ResourceWithImportState = &IncidentDescriptionTemplateResource{}
)

// incidentDescriptionTemplateID is the fixed ID of the organization's only incident description template.
const incidentDescriptionTemplateID = "default"

// NewIncidentDescriptionTemplateResource is a helper to simplify provider implementation.
func NewIncidentDescriptionTemplateResource() resource.Resource {
	return &IncidentDescriptionTemplateResource{}
}

// IncidentDescriptionTemplateResource is the resource implementation.
type IncidentDescriptionTemplateResource struct {
	client *waroom.Client
}

// IncidentDescriptionTemplateResourceModel describes the resource data model.
type IncidentDescriptionTemplateResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Blob types.String `tfsdk:"blob"`
}

// Metadata returns the resource type name.
func (r *IncidentDescriptionTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_incident_description_template"
}

func (r *IncidentDescriptionTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *IncidentDescriptionTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the incident description template of the Waroom organization. " +
			"The template provides the initial description of new incidents.\n\n" +
			"~> **Note:** An organization has exactly one incident description template, so declare this resource at most once. " +
			"Creating this resource overwrites any template already set, and destroying it clears the template. " +
			"To stop managing the template without clearing it, remove the resource from the state with a `removed` block " +
			"and `lifecycle { destroy = false }` (Terraform 1.7+) or with `terraform state rm`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Resource ID. Always `" + incidentDescriptionTemplateID + "`, since the organization has only one template.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"blob": schema.StringAttribute{
				MarkdownDescription: "The content of the incident description template. Must contain a non-whitespace character; destroy the resource to clear the template.",
				Required:            true,
				Validators: []validator.String{
					waroomvalidator.NotBlank(),
				},
			},
		},
	}
}

// Create sets the template and sets the initial Terraform state.
//
// The API has no create operation; the template is upserted with Update.
func (r *IncidentDescriptionTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IncidentDescriptionTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.put(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateIncidentDescriptionTemplate, got error: %s", err))
		return
	}

	tflog.Info(ctx, "created incident description template", map[string]interface{}{
		"id": state.Id.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
//
// The API returns an empty template when none is set, which is treated as absent.
func (r *IncidentDescriptionTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IncidentDescriptionTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tmpl, err := r.client.IncidentDescriptionTemplate.Get(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetIncidentDescriptionTemplate, got error: %s", err))
		return
	}

	if tmpl.Blob == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Id = types.StringValue(incidentDescriptionTemplateID)
	data.Blob = types.StringValue(tmpl.Blob)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the template and sets the updated Terraform state on success.
func (r *IncidentDescriptionTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan IncidentDescriptionTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.put(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateIncidentDescriptionTemplate, got error: %s", err))
		return
	}

	tflog.Info(ctx, "updated incident description template", map[string]interface{}{
		"id": state.Id.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete clears the template and removes the Terraform state on success.
func (r *IncidentDescriptionTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.client.IncidentDescriptionTemplate.Delete(ctx); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("DeleteIncidentDescriptionTemplate, got error: %s", err))
		return
	}
}

// ImportState imports the organization's incident description template into Terraform state.
//
// The organization has only one template, so the import ID is always "default".
//
// Example:
//
//	terraform import waroom_incident_description_template.example default
func (r *IncidentDescriptionTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != incidentDescriptionTemplateID {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID %q. Got: %s", incidentDescriptionTemplateID, req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// put upserts the template from the plan and returns the resulting state.
func (r *IncidentDescriptionTemplateResource) put(ctx context.Context, plan IncidentDescriptionTemplateResourceModel) (IncidentDescriptionTemplateResourceModel, error) {
	// The schema validator is skipped when blob is unknown at plan time, so check it again here:
	// the API treats a blank template as absent, and Read would then drop the resource.
	if waroomvalidator.IsBlank(plan.Blob.ValueString()) {
		return IncidentDescriptionTemplateResourceModel{}, errors.New("blob must contain a non-whitespace character; destroy the resource to clear the template")
	}

	updated, err := r.client.IncidentDescriptionTemplate.Update(ctx, waroom.UpdateIncidentDescriptionTemplateReq{
		Blob: waroom.NewOptString(plan.Blob.ValueString()),
	})
	if err != nil {
		return IncidentDescriptionTemplateResourceModel{}, err
	}

	return IncidentDescriptionTemplateResourceModel{
		Id:   types.StringValue(incidentDescriptionTemplateID),
		Blob: types.StringValue(updated.Blob),
	}, nil
}
