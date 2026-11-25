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
	_ resource.Resource                = &RunbookResource{}
	_ resource.ResourceWithConfigure   = &RunbookResource{}
	_ resource.ResourceWithImportState = &RunbookResource{}
	_ resource.ResourceWithModifyPlan  = &RunbookResource{}
)

// NewRunbookResource is a helper to simplify provider implementation.
func NewRunbookResource() resource.Resource {
	return &RunbookResource{}
}

// RunbookResource is the resource implementation.
type RunbookResource struct {
	client *waroom.Client
}

// RunbookResourceModel describes the resource data model.
type RunbookResourceModel struct {
	Id        types.String `tfsdk:"id"`
	Namespace types.String `tfsdk:"namespace"`
	Blob      types.String `tfsdk:"blob"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

// Metadata returns the resource type name.
func (r *RunbookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runbook"
}

func (r *RunbookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *RunbookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Waroom runbook. A runbook is Markdown content identified by its path-style namespace " +
			"(e.g. `/default`, `/api-server/recovery`).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				// Set from namespace in ModifyPlan.
				MarkdownDescription: "Runbook ID. This is an alias for `namespace`.",
				Computed:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "The path-style namespace of the runbook, e.g. `/api-server/recovery`. It must start with `/`, " +
					"may contain only ASCII letters, digits, `_`, `-` and `/`, and is at most 255 characters. " +
					"The root namespace `/` is not supported. Changing it renames the runbook in place.",
				Required:   true,
				Validators: runbookNamespaceWithSlashValidators(),
			},
			"blob": schema.StringAttribute{
				MarkdownDescription: "Markdown content of the runbook. An empty string is not allowed.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The time the runbook was created, in RFC 3339 format.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The time the runbook was last updated, in RFC 3339 format.",
				Computed:            true,
			},
		},
	}
}

// ModifyPlan keeps id equal to namespace, so a rename shows the new id in the plan
// instead of carrying over the old one from state.
func (r *RunbookResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var namespace types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("namespace"), &namespace)...)
	if resp.Diagnostics.HasError() || namespace.IsUnknown() {
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), namespace)...)
}

// Create creates the resource and sets the initial Terraform state.
func (r *RunbookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RunbookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.Runbooks.Create(ctx, waroom.CreateRunbookReq{
		Namespace: plan.Namespace.ValueString(),
		Blob:      plan.Blob.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("CreateRunbook, got error: %s", err))
		return
	}

	var state RunbookResourceModel
	state.setFromAPI(created.Namespace, created.Blob, created.CreatedAt, created.UpdatedAt)

	tflog.Info(ctx, "created runbook", map[string]interface{}{
		"id":        state.Id.ValueString(),
		"namespace": state.Namespace.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *RunbookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RunbookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	runbook, err := r.client.Runbooks.Get(ctx, waroom.GetRunbookParams{
		Namespace: runbookNamespaceParam(data.Namespace.ValueString()),
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetRunbook, got error: %s", err))
		return
	}

	data.setFromAPI(runbook.Namespace, runbook.Blob, runbook.CreatedAt, runbook.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource and sets the updated Terraform state on success.
// A namespace change renames the runbook in place.
func (r *RunbookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RunbookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state RunbookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var ureq waroom.UpdateRunbookReq
	if plan.Blob.ValueString() != state.Blob.ValueString() {
		ureq.Blob.SetTo(plan.Blob.ValueString())
	}
	if plan.Namespace.ValueString() != state.Namespace.ValueString() {
		ureq.Namespace.SetTo(plan.Namespace.ValueString())
	}

	updated, err := r.client.Runbooks.Update(ctx, ureq, waroom.UpdateRunbookParams{
		Namespace: runbookNamespaceParam(state.Namespace.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateRunbook, got error: %s", err))
		return
	}

	var newState RunbookResourceModel
	newState.setFromAPI(updated.Namespace, updated.Blob, updated.CreatedAt, updated.UpdatedAt)

	tflog.Info(ctx, "updated runbook", map[string]interface{}{
		"id":        newState.Id.ValueString(),
		"namespace": newState.Namespace.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *RunbookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RunbookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Runbooks.Delete(ctx, waroom.DeleteRunbookParams{
		Namespace: runbookNamespaceParam(state.Namespace.ValueString()),
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("DeleteRunbook, got error: %s", err))
		return
	}
}

// ImportState imports an existing runbook into Terraform state.
//
// The import ID is the namespace of the runbook. The leading slash is optional.
//
// Example:
//
//	terraform import waroom_runbook.example /api-server/recovery
func (r *RunbookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	namespace, ok := runbookImportNamespace(ctx, req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected the namespace of a runbook, e.g. /api-server/recovery. Got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), namespace)...)
}

func (m *RunbookResourceModel) setFromAPI(namespace, blob string, createdAt, updatedAt waroom.OptDateTime) {
	m.Id = types.StringValue(namespace)
	m.Namespace = types.StringValue(namespace)
	m.Blob = types.StringValue(blob)
	m.CreatedAt = runbookTimeValue(createdAt.Value, createdAt.Set)
	m.UpdatedAt = runbookTimeValue(updatedAt.Value, updatedAt.Set)
}

// runbookImportNamespace normalizes an import ID to a namespace with a leading slash,
// and reports whether it is a valid namespace for the resource.
func runbookImportNamespace(ctx context.Context, id string) (string, bool) {
	namespace := "/" + runbookNamespaceParam(id)
	for _, v := range runbookNamespaceWithSlashValidators() {
		var resp validator.StringResponse
		v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(namespace)}, &resp)
		if resp.Diagnostics.HasError() {
			return "", false
		}
	}
	return namespace, true
}
