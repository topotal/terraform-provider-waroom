package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	waroom "github.com/topotal/waroom-client-go/waroom"

	waroomvalidator "github.com/topotal/terraform-provider-waroom/internal/validator"
)

var (
	_ resource.Resource                = &OnCallEscalationPolicyResource{}
	_ resource.ResourceWithConfigure   = &OnCallEscalationPolicyResource{}
	_ resource.ResourceWithImportState = &OnCallEscalationPolicyResource{}
)

const (
	onCallEscalationPolicyDefaultTimezone      = "Asia/Tokyo"
	onCallEscalationPolicyDefaultCallTraversal = "depth_first"
	onCallEscalationPolicyDefaultStepType      = "primary"
)

// onCallEscalationPolicyTimeRegexp matches a time of day in HH:MM format.
var onCallEscalationPolicyTimeRegexp = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// onCallEscalationPolicyStepAttrTypes are the attribute types of an element of steps.
var onCallEscalationPolicyStepAttrTypes = map[string]attr.Type{
	"team_uuid":         types.StringType,
	"step_type":         types.StringType,
	"simultaneous_call": types.BoolType,
}

// onCallEscalationPolicyTimeSlotAttrTypes are the attribute types of an element of time_slots.
var onCallEscalationPolicyTimeSlotAttrTypes = map[string]attr.Type{
	"day_of_week": types.Int64Type,
	"start_time":  types.StringType,
	"end_time":    types.StringType,
}

// NewOnCallEscalationPolicyResource is a helper to simplify provider implementation.
func NewOnCallEscalationPolicyResource() resource.Resource {
	return &OnCallEscalationPolicyResource{}
}

// OnCallEscalationPolicyResource is the resource implementation.
type OnCallEscalationPolicyResource struct {
	client *waroom.Client
}

// OnCallEscalationPolicyResourceModel describes the resource data model.
type OnCallEscalationPolicyResourceModel struct {
	Id            types.String `tfsdk:"id"`
	UUID          types.String `tfsdk:"uuid"`
	Name          types.String `tfsdk:"name"`
	Timezone      types.String `tfsdk:"timezone"`
	CallDepth     types.Int64  `tfsdk:"call_depth"`
	CallTraversal types.String `tfsdk:"call_traversal"`
	Steps         types.List   `tfsdk:"steps"`
	TimeSlots     types.Set    `tfsdk:"time_slots"`
}

// OnCallEscalationPolicyResourceStepModel describes an element of steps.
type OnCallEscalationPolicyResourceStepModel struct {
	TeamUUID         types.String `tfsdk:"team_uuid"`
	StepType         types.String `tfsdk:"step_type"`
	SimultaneousCall types.Bool   `tfsdk:"simultaneous_call"`
}

// OnCallEscalationPolicyResourceTimeSlotModel describes an element of time_slots.
type OnCallEscalationPolicyResourceTimeSlotModel struct {
	DayOfWeek types.Int64  `tfsdk:"day_of_week"`
	StartTime types.String `tfsdk:"start_time"`
	EndTime   types.String `tfsdk:"end_time"`
}

// Metadata returns the resource type name.
func (r *OnCallEscalationPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_on_call_escalation_policy"
}

func (r *OnCallEscalationPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *OnCallEscalationPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	timeValidators := []validator.String{
		stringvalidator.RegexMatches(onCallEscalationPolicyTimeRegexp, "must be a time in HH:MM format"),
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Waroom on-call escalation policy. " +
			"An escalation policy defines which on-call teams are called, in which order, and on which days and times.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Escalation policy ID. This is an alias for `uuid`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The unique identifier (UUID) of the escalation policy. This is automatically generated by Waroom.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the escalation policy.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "The timezone (IANA format, e.g. `Asia/Tokyo`) that `time_slots` are interpreted in. " +
					"Defaults to `" + onCallEscalationPolicyDefaultTimezone + "`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(onCallEscalationPolicyDefaultTimezone),
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"call_depth": schema.Int64Attribute{
				MarkdownDescription: "How many people, counted from the on-duty user in the rotation order, are called in each step's team (1 to 10). " +
					"Absent users are not counted; users who cannot be called (e.g. without a phone number) are counted. " +
					"Omit it to call everyone in the team.",
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(1, 10),
				},
			},
			"call_traversal": schema.StringAttribute{
				MarkdownDescription: "The order in which steps and people are called. " +
					"`depth_first` calls up to `call_depth` people in a step before moving on to the next step; " +
					"`breadth_first` calls the on-duty user of every step first, then the second person of every step, and so on. " +
					"Defaults to `" + onCallEscalationPolicyDefaultCallTraversal + "`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(onCallEscalationPolicyDefaultCallTraversal),
				Validators: []validator.String{
					stringvalidator.OneOf("depth_first", "breadth_first"),
				},
			},
			"steps": schema.ListNestedAttribute{
				MarkdownDescription: "The escalation steps, in escalation order. Defaults to no steps.",
				Optional:            true,
				Computed:            true,
				Default: listdefault.StaticValue(types.ListValueMust(
					types.ObjectType{AttrTypes: onCallEscalationPolicyStepAttrTypes}, []attr.Value{},
				)),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"team_uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the on-call team called in this step.",
							Required:            true,
							Validators: []validator.String{
								waroomvalidator.UUID(),
							},
						},
						"step_type": schema.StringAttribute{
							MarkdownDescription: "The type of the step: `primary` or `fallback`. Defaults to `" + onCallEscalationPolicyDefaultStepType + "`.",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString(onCallEscalationPolicyDefaultStepType),
							Validators: []validator.String{
								stringvalidator.OneOf("primary", "fallback"),
							},
						},
						"simultaneous_call": schema.BoolAttribute{
							MarkdownDescription: "Whether all participants of the team's active rotation are called at the same time in this step. Defaults to `false`.",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
						},
					},
				},
			},
			"time_slots": schema.SetNestedAttribute{
				MarkdownDescription: "The days of the week and times when the escalation policy is active. At least one is required.",
				Required:            true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"day_of_week": schema.Int64Attribute{
							MarkdownDescription: "The day of the week (0 is Sunday, 6 is Saturday).",
							Required:            true,
							Validators: []validator.Int64{
								int64validator.Between(0, 6),
							},
						},
						"start_time": schema.StringAttribute{
							MarkdownDescription: "The start time (`HH:MM`) in the policy's `timezone`.",
							Required:            true,
							Validators:          timeValidators,
						},
						"end_time": schema.StringAttribute{
							MarkdownDescription: "The end time (`HH:MM`) in the policy's `timezone`. " +
								"If it is earlier than `start_time`, the slot continues into the next day. `23:59` includes the end of the day.",
							Required:   true,
							Validators: timeValidators,
						},
					},
				},
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *OnCallEscalationPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OnCallEscalationPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	steps, timeSlots, diags := plan.expand(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	creq := waroom.CreateOnCallEscalationPolicyReq{
		Name:      plan.Name.ValueString(),
		Timezone:  waroom.NewOptString(plan.Timezone.ValueString()),
		Steps:     make([]waroom.CreateOnCallEscalationPolicyStep, 0, len(steps)),
		TimeSlots: make([]waroom.CreateOnCallEscalationPolicyTimeSlot, 0, len(timeSlots)),
	}
	if plan.CallDepth.IsNull() {
		creq.CallDepth.SetToNull()
	} else {
		creq.CallDepth.SetTo(int(plan.CallDepth.ValueInt64()))
	}
	creq.CallTraversal.SetTo(waroom.CreateOnCallEscalationPolicyCallTraversal(plan.CallTraversal.ValueString()))
	for _, step := range steps {
		s := waroom.CreateOnCallEscalationPolicyStep{TeamUUID: step.TeamUUID.ValueString()}
		s.StepType.SetTo(waroom.CreateOnCallEscalationPolicyStepType(step.StepType.ValueString()))
		s.SimultaneousCall.SetTo(step.SimultaneousCall.ValueBool())
		creq.Steps = append(creq.Steps, s)
	}
	for _, slot := range timeSlots {
		creq.TimeSlots = append(creq.TimeSlots, waroom.CreateOnCallEscalationPolicyTimeSlot{
			DayOfWeek: int(slot.DayOfWeek.ValueInt64()),
			StartTime: slot.StartTime.ValueString(),
			EndTime:   slot.EndTime.ValueString(),
		})
	}

	created, err := r.client.OnCallEscalationPolicies.Create(ctx, creq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("CreateOnCallEscalationPolicy, got error: %s", err))
		return
	}

	// The create response has the same shape as the get response.
	policy, err := toOnCallEscalationPolicy(created)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("CreateOnCallEscalationPolicy, unexpected response: %s", err))
		return
	}

	state, diags := newOnCallEscalationPolicyResourceModel(policy)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "created on-call escalation policy", map[string]interface{}{
		"id":   state.Id.ValueString(),
		"uuid": state.UUID.ValueString(),
		"name": state.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *OnCallEscalationPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OnCallEscalationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := r.client.OnCallEscalationPolicies.Get(ctx, waroom.GetOnCallEscalationPolicyParams{UUID: data.UUID.ValueString()})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetOnCallEscalationPolicy, got error: %s", err))
		return
	}

	state, diags := newOnCallEscalationPolicyResourceModel(policy)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
//
// Every attribute is sent, so that steps and time_slots are replaced as a whole and
// call_depth is reset to unlimited when it is removed from the configuration.
func (r *OnCallEscalationPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OnCallEscalationPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state OnCallEscalationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	steps, timeSlots, diags := plan.expand(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ureq := waroom.UpdateOnCallEscalationPolicyReq{
		Name:     waroom.NewOptString(plan.Name.ValueString()),
		Timezone: waroom.NewOptString(plan.Timezone.ValueString()),
		// Non-nil so that an empty list is sent and clears the steps.
		Steps:     make([]waroom.UpdateOnCallEscalationPolicyStep, 0, len(steps)),
		TimeSlots: make([]waroom.UpdateOnCallEscalationPolicyTimeSlot, 0, len(timeSlots)),
	}
	if plan.CallDepth.IsNull() {
		ureq.CallDepth.SetToNull()
	} else {
		ureq.CallDepth.SetTo(int(plan.CallDepth.ValueInt64()))
	}
	ureq.CallTraversal.SetTo(waroom.UpdateOnCallEscalationPolicyCallTraversal(plan.CallTraversal.ValueString()))
	for _, step := range steps {
		s := waroom.UpdateOnCallEscalationPolicyStep{TeamUUID: step.TeamUUID.ValueString()}
		s.StepType.SetTo(waroom.UpdateOnCallEscalationPolicyStepType(step.StepType.ValueString()))
		s.SimultaneousCall.SetTo(step.SimultaneousCall.ValueBool())
		ureq.Steps = append(ureq.Steps, s)
	}
	for _, slot := range timeSlots {
		ureq.TimeSlots = append(ureq.TimeSlots, waroom.UpdateOnCallEscalationPolicyTimeSlot{
			DayOfWeek: int(slot.DayOfWeek.ValueInt64()),
			StartTime: slot.StartTime.ValueString(),
			EndTime:   slot.EndTime.ValueString(),
		})
	}

	updated, err := r.client.OnCallEscalationPolicies.Update(ctx, ureq, waroom.UpdateOnCallEscalationPolicyParams{UUID: state.UUID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateOnCallEscalationPolicy, got error: %s", err))
		return
	}

	// The update response has the same shape as the get response.
	policy, err := toOnCallEscalationPolicy(updated)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateOnCallEscalationPolicy, unexpected response: %s", err))
		return
	}

	newState, diags := newOnCallEscalationPolicyResourceModel(policy)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "updated on-call escalation policy", map[string]interface{}{
		"id":   newState.Id.ValueString(),
		"uuid": newState.UUID.ValueString(),
		"name": newState.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *OnCallEscalationPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OnCallEscalationPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.OnCallEscalationPolicies.Delete(ctx, waroom.DeleteOnCallEscalationPolicyParams{UUID: state.UUID.ValueString()})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("DeleteOnCallEscalationPolicy, got error: %s", err))
		return
	}
}

// ImportState imports an existing on-call escalation policy into Terraform state.
//
// The import ID is the UUID of the escalation policy.
//
// Example:
//
//	terraform import waroom_on_call_escalation_policy.example 123e4567-e89b-12d3-a456-426614174000
func (r *OnCallEscalationPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := uuid.Parse(req.ID); err != nil {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected the UUID of an on-call escalation policy. Got: %s", req.ID),
		)
		return
	}

	resource.ImportStatePassthroughID(ctx, path.Root("uuid"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// expand converts steps and time_slots of the model into Go values.
func (m *OnCallEscalationPolicyResourceModel) expand(ctx context.Context) ([]OnCallEscalationPolicyResourceStepModel, []OnCallEscalationPolicyResourceTimeSlotModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var steps []OnCallEscalationPolicyResourceStepModel
	var timeSlots []OnCallEscalationPolicyResourceTimeSlotModel
	if !m.Steps.IsNull() && !m.Steps.IsUnknown() {
		diags.Append(m.Steps.ElementsAs(ctx, &steps, false)...)
	}
	if !m.TimeSlots.IsNull() && !m.TimeSlots.IsUnknown() {
		diags.Append(m.TimeSlots.ElementsAs(ctx, &timeSlots, false)...)
	}
	return steps, timeSlots, diags
}

// toOnCallEscalationPolicy converts a create / update response into the get response type,
// which has the same JSON shape, so that a single mapping function can be used.
func toOnCallEscalationPolicy(v interface{ MarshalJSON() ([]byte, error) }) (*waroom.OnCallEscalationPolicy, error) {
	b, err := v.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var policy waroom.OnCallEscalationPolicy
	if err := policy.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return &policy, nil
}

// newOnCallEscalationPolicyResourceModel converts an API escalation policy into the resource model.
func newOnCallEscalationPolicyResourceModel(policy *waroom.OnCallEscalationPolicy) (OnCallEscalationPolicyResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	data := OnCallEscalationPolicyResourceModel{
		Id:            types.StringValue(policy.UUID),
		UUID:          types.StringValue(policy.UUID),
		Name:          types.StringValue(policy.Name),
		Timezone:      types.StringValue(policy.Timezone),
		CallDepth:     types.Int64Null(),
		CallTraversal: types.StringValue(string(policy.CallTraversal)),
	}
	if depth, ok := policy.CallDepth.Get(); ok {
		data.CallDepth = types.Int64Value(int64(depth))
	}

	stepType := types.ObjectType{AttrTypes: onCallEscalationPolicyStepAttrTypes}
	steps := make([]attr.Value, 0, len(policy.Steps))
	for _, step := range policy.Steps {
		obj, d := types.ObjectValue(onCallEscalationPolicyStepAttrTypes, map[string]attr.Value{
			"team_uuid":         types.StringValue(step.Team.UUID),
			"step_type":         types.StringValue(string(step.StepType)),
			"simultaneous_call": types.BoolValue(step.SimultaneousCall),
		})
		diags.Append(d...)
		steps = append(steps, obj)
	}
	var d diag.Diagnostics
	data.Steps, d = types.ListValue(stepType, steps)
	diags.Append(d...)

	slotType := types.ObjectType{AttrTypes: onCallEscalationPolicyTimeSlotAttrTypes}
	slots := make([]attr.Value, 0, len(policy.TimeSlots))
	for _, slot := range policy.TimeSlots {
		obj, d := types.ObjectValue(onCallEscalationPolicyTimeSlotAttrTypes, map[string]attr.Value{
			"day_of_week": types.Int64Value(int64(slot.DayOfWeek)),
			"start_time":  types.StringValue(slot.StartTime),
			"end_time":    types.StringValue(slot.EndTime),
		})
		diags.Append(d...)
		slots = append(slots, obj)
	}
	data.TimeSlots, d = types.SetValue(slotType, slots)
	diags.Append(d...)

	return data, diags
}
