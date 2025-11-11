package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource              = &ServiceLabelResource{}
	_ resource.ResourceWithConfigure = &ServiceLabelResource{}
)

const (
	DefaultColor string = "008BD1"
)

// NewServiceLabelResource is a helper to simplify provider implementation.
func NewServiceLabelResource() resource.Resource {
	return &ServiceLabelResource{}
}

// ServiceLabelResource is the resource implementation.
type ServiceLabelResource struct {
	client *waroom.Client
}

// ServiceLabelResourceModel describes the resource data model.
type ServiceLabelResourceModel struct {
	Id          types.String `tfsdk:"id"`
	ServiceName types.String `tfsdk:"service_name"`
	UUID        types.String `tfsdk:"uuid"`
	Name        types.String `tfsdk:"name"`
	Color       types.String `tfsdk:"color"`
}

// Metadata returns the resource type name.
func (r *ServiceLabelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_label"
}

func (r *ServiceLabelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *ServiceLabelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// TODO: write docs
		MarkdownDescription: "Waroom service label resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Label ID. This is an alias for uuid.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "Label UUID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_name": schema.StringAttribute{
				MarkdownDescription: "The name of the service to which the label belongs.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Label name.",
				Required:            true,
			},
			"color": schema.StringAttribute{
				MarkdownDescription: "Label color in hex format(without#).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-fA-F]{6}$`),
						"must be a 6-digit hex color code without #"),
				},
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *ServiceLabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceLabelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	svc := waroom.NewLabelService(r.client)

	// POST /:service/labels
	params := waroom.CreateServiceLabelParams{
		Name: plan.ServiceName.ValueString(), // ここのNameはサービス名
	}

	wreq := waroom.CreateServiceLabelReq{
		Name:  plan.Name.ValueString(),
		Color: plan.Color.ValueString(),
	}
	created, err := svc.CreateServiceLabel(ctx, wreq, params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("CreateServiceLabel, got error: %s", err))
		//TODO: 409 conflictだった場合に terraform import 可能なヒントを提示
		return
	}

	var state ServiceLabelResourceModel
	state.Id = types.StringValue(created.UUID.String())
	state.UUID = types.StringValue(created.UUID.String())
	state.Color = types.StringValue(created.Color.Value)
	state.Name = types.StringValue(created.Name)
	state.ServiceName = plan.ServiceName

	tflog.Info(ctx, "created service label", map[string]interface{}{
		"id":    state.UUID.ValueString(),
		"uuid":  state.UUID.ValueString(),
		"name":  state.Name.ValueString(),
		"color": state.Color.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *ServiceLabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceLabelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "mapping", map[string]interface{}{
		"uuid":  data.UUID.ValueString(),
		"name":  data.Name.ValueString(),
		"color": data.Color.ValueString(),
	})

	svc := waroom.NewLabelService(r.client)
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
			data.Name = types.StringValue(label.Name)
			// data.UpdateAt = types.StringValue(label.UpdatedAt.Value.String())
		}
	}

	if data.UUID.IsNull() {
		resp.Diagnostics.AddWarning("Not Found", "The service label was not found.")
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *ServiceLabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceLabelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state ServiceLabelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	svc := waroom.NewLabelService(r.client)

	uuid, err := uuid.Parse(state.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid UUID",
			fmt.Sprintf("Unable to parse UUID from state: %s", err),
		)
		return
	}

	params := waroom.UpdateServiceLabelParams{
		Name: plan.ServiceName.ValueString(),
		UUID: uuid,
	}

	ureq := waroom.UpdateServiceLabelReq{
		Name:  waroom.NewOptString(plan.Name.ValueString()),
		Color: waroom.NewOptString(plan.Color.ValueString()),
	}
	created, err := svc.UpdateServiceLabel(ctx, ureq, params)
	if err != nil {
		resp.Diagnostics.AddError(
			"Update Error",
			fmt.Sprintf("Unable to create service label during update: %s", err),
		)
		return
	}

	// Update state with new values
	var newState ServiceLabelResourceModel
	newState.Id = types.StringValue(created.UUID.String())
	newState.UUID = types.StringValue(created.UUID.String())
	newState.Color = types.StringValue(created.Color.Value)
	newState.Name = types.StringValue(created.Name)
	newState.ServiceName = plan.ServiceName

	tflog.Info(ctx, "created new service label during update", map[string]interface{}{
		"id":    newState.UUID.ValueString(),
		"uuid":  newState.UUID.ValueString(),
		"name":  newState.Name.ValueString(),
		"color": newState.Color.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *ServiceLabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServiceLabelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	svc := waroom.NewLabelService(r.client)
	uuid, err := uuid.Parse(state.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("invalid UUID value", fmt.Sprintf("got error: %s", err))
		return
	}

	params := waroom.DeleteServiceLabelParams{
		Name: state.ServiceName.ValueString(),
		UUID: uuid,
	}
	err = svc.DeleteServiceLabel(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Waroom Client Error", fmt.Sprintf("DeleteServiceLabel, got error: %s", err))
		return
	}
}
