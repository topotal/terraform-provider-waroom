package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/topotal/waroom-client-go/waroom"

	wvalidator "github.com/topotal/terraform-provider-waroom/internal/validator"
)

var (
	_ resource.Resource                   = &OnCallTeamResource{}
	_ resource.ResourceWithConfigure      = &OnCallTeamResource{}
	_ resource.ResourceWithImportState    = &OnCallTeamResource{}
	_ resource.ResourceWithValidateConfig = &OnCallTeamResource{}
)

const (
	// onCallTeamDefaultIntervalDays is the rotation interval the API uses when interval_days is omitted.
	onCallTeamDefaultIntervalDays = 7

	// onCallTeamStartedAtPrivateKey is the private state key holding the started_at values
	// returned by the API, keyed by rotation UUID. See onCallTeamStartedAt.
	onCallTeamStartedAtPrivateKey = "rotation_started_at"
)

var hhmmRegex = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

var onCallTeamTimeSlotAttrTypes = map[string]attr.Type{
	"day_of_week": types.Int64Type,
	"start_time":  types.StringType,
	"end_time":    types.StringType,
}

// NewOnCallTeamResource is a helper to simplify provider implementation.
func NewOnCallTeamResource() resource.Resource {
	return &OnCallTeamResource{}
}

// OnCallTeamResource is the resource implementation.
type OnCallTeamResource struct {
	client *waroom.Client
}

// OnCallTeamResourceModel describes the resource data model.
type OnCallTeamResourceModel struct {
	Id        types.String              `tfsdk:"id"`
	UUID      types.String              `tfsdk:"uuid"`
	Name      types.String              `tfsdk:"name"`
	Color     types.String              `tfsdk:"color"`
	Rotations []OnCallTeamRotationModel `tfsdk:"rotations"`
}

// OnCallTeamRotationModel describes a rotation of an on-call team.
type OnCallTeamRotationModel struct {
	UUID                types.String `tfsdk:"uuid"`
	Timezone            types.String `tfsdk:"timezone"`
	StartedAt           types.String `tfsdk:"started_at"`
	ShiftHandoffTime    types.String `tfsdk:"shift_handoff_time"`
	ShiftHandoffWday    types.Int64  `tfsdk:"shift_handoff_wday"`
	IntervalDays        types.Int64  `tfsdk:"interval_days"`
	ParticipantUserSubs types.List   `tfsdk:"participant_user_subs"`
	TimeSlots           types.Set    `tfsdk:"time_slots"`
}

// OnCallTeamTimeSlotModel describes a time slot of a rotation.
type OnCallTeamTimeSlotModel struct {
	DayOfWeek types.Int64  `tfsdk:"day_of_week"`
	StartTime types.String `tfsdk:"start_time"`
	EndTime   types.String `tfsdk:"end_time"`
}

// Metadata returns the resource type name.
func (r *OnCallTeamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_on_call_team"
}

func (r *OnCallTeamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *OnCallTeamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	hhmm := stringvalidator.RegexMatches(hhmmRegex, "must be a time in HH:MM format (00:00-23:59)")

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Waroom on-call team. An on-call team has one or more rotations " +
			"(e.g. one for weekdays and one for weekends) that decide who is on duty.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "On-call team ID. This is an alias for `uuid`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The unique identifier (UUID) of the on-call team. This is automatically generated by Waroom.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the on-call team.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"color": schema.StringAttribute{
				MarkdownDescription: "The color of the on-call team, as a 6-digit hex code without `#` (e.g. `FF0000`).",
				Required:            true,
				Validators: []validator.String{
					wvalidator.Hex6NoHash(),
				},
			},
			"rotations": schema.ListNestedAttribute{
				MarkdownDescription: "The rotations of the team. At least one is required. The list order is the priority: " +
					"when the active times of rotations overlap, the on-duty participant of the earlier rotation is called. " +
					"Rotations are matched to the existing ones by their position in the list, so removing or reordering " +
					"rotations updates the existing rotations in place.",
				Required: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the rotation.",
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								// A rotation added to the end of the list has no prior state; keep it unknown.
								stringplanmodifier.UseNonNullStateForUnknown(),
							},
						},
						"timezone": schema.StringAttribute{
							MarkdownDescription: "The timezone of the rotation, in IANA format (e.g. `Asia/Tokyo`). " +
								"`shift_handoff_time` and `time_slots` are interpreted in this timezone.",
							Required: true,
							Validators: []validator.String{
								wvalidator.Timezone(),
							},
						},
						"started_at": schema.StringAttribute{
							MarkdownDescription: "The date and time the first shift starts, in RFC 3339 format (e.g. `2026-01-05T10:00:00+09:00`). " +
								"Waroom adjusts it to a shift handoff: when `interval_days` is a multiple of 7, to the latest handoff " +
								"(`shift_handoff_wday` and `shift_handoff_time`) at or before this time; otherwise, to `shift_handoff_time` on the same day. " +
								"The configured value is kept in the state, and a change is shown only when the start is changed outside Terraform.",
							Required: true,
						},
						"shift_handoff_time": schema.StringAttribute{
							MarkdownDescription: "The time of day (`HH:MM`) when the shift is handed off to the next participant.",
							Required:            true,
							Validators:          []validator.String{hhmm},
						},
						"shift_handoff_wday": schema.Int64Attribute{
							MarkdownDescription: "The day of the week the shift is handed off (0 is Sunday, 6 is Saturday). " +
								"Required when `interval_days` is a multiple of 7, and must be omitted otherwise.",
							Optional: true,
							Validators: []validator.Int64{
								int64validator.Between(0, 6),
							},
						},
						"interval_days": schema.Int64Attribute{
							MarkdownDescription: fmt.Sprintf("The number of days between shift handoffs (1-14). Defaults to `%d` (weekly).", onCallTeamDefaultIntervalDays),
							Optional:            true,
							Computed:            true,
							Default:             int64default.StaticInt64(onCallTeamDefaultIntervalDays),
							Validators: []validator.Int64{
								int64validator.Between(1, 14),
							},
						},
						"participant_user_subs": schema.ListAttribute{
							MarkdownDescription: "The `sub` of the users in the rotation, in on-duty order. " +
								"Only users who have verified their phone number can be added. Defaults to no participants.",
							ElementType: types.StringType,
							Optional:    true,
							Computed:    true,
							Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
							Validators: []validator.List{
								listvalidator.UniqueValues(),
								listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
							},
						},
						"time_slots": schema.SetNestedAttribute{
							MarkdownDescription: "The days of the week and times when the rotation is active. Defaults to none, which means always active.",
							Optional:            true,
							Computed:            true,
							Default:             setdefault.StaticValue(types.SetValueMust(types.ObjectType{AttrTypes: onCallTeamTimeSlotAttrTypes}, []attr.Value{})),
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
										MarkdownDescription: "The start time (`HH:MM`).",
										Required:            true,
										Validators:          []validator.String{hhmm},
									},
									"end_time": schema.StringAttribute{
										MarkdownDescription: "The end time (`HH:MM`). A time earlier than `start_time` continues into the next day " +
											"(e.g. `22:00`-`02:00`), and `23:59` includes the end of the day.",
										Required:   true,
										Validators: []validator.String{hhmm},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// ValidateConfig checks the rules between rotation attributes that the schema cannot express.
func (r *OnCallTeamResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var rotations types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("rotations"), &rotations)...)
	if resp.Diagnostics.HasError() || rotations.IsNull() || rotations.IsUnknown() {
		return
	}

	// Convert each rotation separately: a rotation can be unknown as a whole (e.g. chosen by a condition
	// on another resource's attribute), and it is then checked at apply time instead.
	var objects []types.Object
	resp.Diagnostics.Append(rotations.ElementsAs(ctx, &objects, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for i, obj := range objects {
		if obj.IsNull() || obj.IsUnknown() {
			continue
		}
		var m OnCallTeamRotationModel
		resp.Diagnostics.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		p := path.Root("rotations").AtListIndex(i)
		if !m.StartedAt.IsNull() && !m.StartedAt.IsUnknown() {
			if _, err := time.Parse(time.RFC3339, m.StartedAt.ValueString()); err != nil {
				resp.Diagnostics.AddAttributeError(p.AtName("started_at"), "Invalid Attribute Value",
					fmt.Sprintf("started_at must be in RFC 3339 format (e.g. 2026-01-05T10:00:00+09:00), got: %q", m.StartedAt.ValueString()))
			}
		}
		if msg := validateOnCallTeamShiftHandoffWday(m.IntervalDays, m.ShiftHandoffWday); msg != "" {
			resp.Diagnostics.AddAttributeError(p.AtName("shift_handoff_wday"), "Invalid Attribute Combination", msg)
		}
	}
}

// validateOnCallTeamShiftHandoffWday returns an error message when shift_handoff_wday does not fit interval_days,
// or "" when it is valid or cannot be checked yet. A null interval_days means the default.
func validateOnCallTeamShiftHandoffWday(intervalDays, wday types.Int64) string {
	if intervalDays.IsUnknown() || wday.IsUnknown() {
		return ""
	}
	interval := int64(onCallTeamDefaultIntervalDays)
	if !intervalDays.IsNull() {
		interval = intervalDays.ValueInt64()
	}
	weekly := interval%7 == 0
	switch {
	case weekly && wday.IsNull():
		return fmt.Sprintf("shift_handoff_wday is required when interval_days (%d) is a multiple of 7.", interval)
	case !weekly && !wday.IsNull():
		return fmt.Sprintf("shift_handoff_wday must be omitted when interval_days (%d) is not a multiple of 7, "+
			"because the handoff day of the week changes every time.", interval)
	}
	return ""
}

// Create creates the resource and sets the initial Terraform state.
func (r *OnCallTeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OnCallTeamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rotations, diags := onCallTeamRotationsRequest(ctx, plan.Rotations)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	creq := waroom.CreateOnCallTeamReq{
		Name:      plan.Name.ValueString(),
		Color:     plan.Color.ValueString(),
		Rotations: make([]waroom.CreateOnCallTeamRotationReq, len(rotations)),
	}
	for i, rot := range rotations {
		creq.Rotations[i] = toCreateOnCallTeamRotationReq(rot)
	}

	created, err := r.client.OnCallTeams.Create(ctx, creq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("CreateOnCallTeam, got error: %s", err))
		return
	}

	// Save the ID first so that the team is tracked (and tainted) even if reading it back fails.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), created.UUID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("uuid"), created.UUID)...)

	team, err := r.client.OnCallTeams.Get(ctx, waroom.GetOnCallTeamParams{UUID: created.UUID})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetOnCallTeam, got error: %s", err))
		return
	}

	state, recorded, diags := onCallTeamStateFromAPI(ctx, team, plan.Rotations, nil, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "created on-call team", map[string]interface{}{
		"id":   state.Id.ValueString(),
		"uuid": state.UUID.ValueString(),
		"name": state.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	resp.Diagnostics.Append(setOnCallTeamStartedAtPrivate(ctx, resp.Private, recorded)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *OnCallTeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OnCallTeamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	team, err := r.client.OnCallTeams.Get(ctx, waroom.GetOnCallTeamParams{UUID: data.UUID.ValueString()})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetOnCallTeam, got error: %s", err))
		return
	}

	recordedBefore, diags := getOnCallTeamStartedAtPrivate(ctx, req.Private)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state, recorded, diags := onCallTeamStateFromAPI(ctx, team, data.Rotations, recordedBefore, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	resp.Diagnostics.Append(setOnCallTeamStartedAtPrivate(ctx, resp.Private, recorded)...)
}

// Update updates the resource and sets the updated Terraform state on success.
//
// All rotations are sent, which replaces the rotations of the team: a rotation with a UUID is updated,
// one without is added, and rotations missing from the request are deleted.
func (r *OnCallTeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state OnCallTeamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rotations, diags := onCallTeamRotationsRequest(ctx, plan.Rotations)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ureq := waroom.UpdateOnCallTeamReq{
		Name:      waroom.NewOptString(plan.Name.ValueString()),
		Color:     waroom.NewOptString(plan.Color.ValueString()),
		Rotations: rotations,
	}
	uuid := state.UUID.ValueString()
	if _, err := r.client.OnCallTeams.Update(ctx, ureq, waroom.UpdateOnCallTeamParams{UUID: uuid}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("UpdateOnCallTeam, got error: %s", err))
		return
	}

	team, err := r.client.OnCallTeams.Get(ctx, waroom.GetOnCallTeamParams{UUID: uuid})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetOnCallTeam, got error: %s", err))
		return
	}

	newState, recorded, diags := onCallTeamStateFromAPI(ctx, team, plan.Rotations, nil, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "updated on-call team", map[string]interface{}{
		"id":   newState.Id.ValueString(),
		"uuid": newState.UUID.ValueString(),
		"name": newState.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
	resp.Diagnostics.Append(setOnCallTeamStartedAtPrivate(ctx, resp.Private, recorded)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *OnCallTeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OnCallTeamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.OnCallTeams.Delete(ctx, waroom.DeleteOnCallTeamParams{UUID: state.UUID.ValueString()})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("DeleteOnCallTeam, got error: %s", err))
		return
	}
}

// ImportState imports an existing on-call team into Terraform state.
//
// The import ID is the UUID of the on-call team.
// After import, each rotation's started_at is the value adjusted by Waroom.
//
// Example:
//
//	terraform import waroom_on_call_team.example 123e4567-e89b-12d3-a456-426614174000
func (r *OnCallTeamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := parseUUIDImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("uuid"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// onCallTeamRotationsRequest builds the rotations of a create / update request from the planned rotations.
// A rotation keeps its UUID (and so its history) when the plan knows it.
func onCallTeamRotationsRequest(ctx context.Context, rotations []OnCallTeamRotationModel) ([]waroom.UpdateOnCallTeamRotationReq, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make([]waroom.UpdateOnCallTeamRotationReq, 0, len(rotations))
	for _, m := range rotations {
		rot := waroom.UpdateOnCallTeamRotationReq{
			Timezone:         m.Timezone.ValueString(),
			StartedAt:        m.StartedAt.ValueString(),
			ShiftHandoffTime: m.ShiftHandoffTime.ValueString(),
			IntervalDays:     waroom.NewOptInt(int(m.IntervalDays.ValueInt64())),
			Participants:     []waroom.UpdateOnCallTeamParticipantReq{},
			TimeSlots:        []waroom.UpdateOnCallTeamTimeSlotReq{},
		}
		if !m.UUID.IsNull() && !m.UUID.IsUnknown() {
			rot.UUID = waroom.NewOptString(m.UUID.ValueString())
		}
		if m.ShiftHandoffWday.IsNull() {
			rot.ShiftHandoffWday.SetToNull()
		} else {
			rot.ShiftHandoffWday.SetTo(int(m.ShiftHandoffWday.ValueInt64()))
		}

		var subs []string
		diags.Append(m.ParticipantUserSubs.ElementsAs(ctx, &subs, false)...)
		for i, sub := range subs {
			rot.Participants = append(rot.Participants, waroom.UpdateOnCallTeamParticipantReq{UserSub: sub, Position: i})
		}

		var slots []OnCallTeamTimeSlotModel
		diags.Append(m.TimeSlots.ElementsAs(ctx, &slots, false)...)
		for _, s := range slots {
			rot.TimeSlots = append(rot.TimeSlots, waroom.UpdateOnCallTeamTimeSlotReq{
				DayOfWeek: int(s.DayOfWeek.ValueInt64()),
				StartTime: s.StartTime.ValueString(),
				EndTime:   s.EndTime.ValueString(),
			})
		}
		out = append(out, rot)
	}
	return out, diags
}

func toCreateOnCallTeamRotationReq(rot waroom.UpdateOnCallTeamRotationReq) waroom.CreateOnCallTeamRotationReq {
	out := waroom.CreateOnCallTeamRotationReq{
		UUID:             rot.UUID,
		Timezone:         rot.Timezone,
		StartedAt:        rot.StartedAt,
		ShiftHandoffTime: rot.ShiftHandoffTime,
		ShiftHandoffWday: rot.ShiftHandoffWday,
		IntervalDays:     rot.IntervalDays,
		Participants:     make([]waroom.CreateOnCallTeamParticipantReq, len(rot.Participants)),
		TimeSlots:        make([]waroom.CreateOnCallTeamTimeSlotReq, len(rot.TimeSlots)),
	}
	for i, p := range rot.Participants {
		out.Participants[i] = waroom.CreateOnCallTeamParticipantReq(p)
	}
	for i, s := range rot.TimeSlots {
		out.TimeSlots[i] = waroom.CreateOnCallTeamTimeSlotReq(s)
	}
	return out
}

// onCallTeamStateFromAPI maps an on-call team to the resource model.
//
// prior is the planned rotations after create / update, matched by position (the plan has no UUID for an
// added rotation, and the API returns the rotations in request order), or the current rotations in read,
// matched by UUID (rotations added or reordered outside Terraform shift the positions).
// It also returns the started_at values returned by the API, to be recorded in private state.
// See onCallTeamStartedAt for how started_at is chosen.
func onCallTeamStateFromAPI(
	ctx context.Context,
	team *waroom.OnCallTeam,
	prior []OnCallTeamRotationModel,
	recordedBefore map[string]time.Time,
	afterWrite bool,
) (OnCallTeamResourceModel, map[string]time.Time, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := OnCallTeamResourceModel{
		Id:        types.StringValue(team.UUID),
		UUID:      types.StringValue(team.UUID),
		Name:      types.StringValue(team.Name),
		Color:     types.StringValue(team.Color),
		Rotations: make([]OnCallTeamRotationModel, 0, len(team.Rotations)),
	}
	recorded := make(map[string]time.Time, len(team.Rotations))

	priorByUUID := make(map[string]*OnCallTeamRotationModel, len(prior))
	for i := range prior {
		if !prior[i].UUID.IsNull() && !prior[i].UUID.IsUnknown() {
			priorByUUID[prior[i].UUID.ValueString()] = &prior[i]
		}
	}

	for i, rot := range team.Rotations {
		var p *OnCallTeamRotationModel
		switch {
		case !afterWrite:
			p = priorByUUID[rot.UUID]
		case i < len(prior):
			p = &prior[i]
		}

		m := OnCallTeamRotationModel{
			UUID:             types.StringValue(rot.UUID),
			Timezone:         types.StringValue(rot.Timezone),
			StartedAt:        types.StringValue(onCallTeamStartedAt(rot.UUID, rot.StartedAt, p, recordedBefore, afterWrite)),
			ShiftHandoffTime: types.StringValue(rot.ShiftHandoffTime),
			ShiftHandoffWday: types.Int64Null(),
			IntervalDays:     types.Int64Value(int64(rot.IntervalDays)),
		}
		if wday, ok := rot.ShiftHandoffWday.Get(); ok {
			m.ShiftHandoffWday = types.Int64Value(int64(wday))
		}
		recorded[rot.UUID] = rot.StartedAt

		subs := make([]string, 0, len(rot.Participants))
		for _, participant := range rot.Participants {
			subs = append(subs, participant.User.Sub)
		}
		var d diag.Diagnostics
		m.ParticipantUserSubs, d = types.ListValueFrom(ctx, types.StringType, subs)
		diags.Append(d...)

		slots := make([]OnCallTeamTimeSlotModel, 0, len(rot.TimeSlots))
		for _, s := range rot.TimeSlots {
			slots = append(slots, OnCallTeamTimeSlotModel{
				DayOfWeek: types.Int64Value(int64(s.DayOfWeek)),
				StartTime: types.StringValue(s.StartTime),
				EndTime:   types.StringValue(s.EndTime),
			})
		}
		m.TimeSlots, d = types.SetValueFrom(ctx, types.ObjectType{AttrTypes: onCallTeamTimeSlotAttrTypes}, slots)
		diags.Append(d...)

		state.Rotations = append(state.Rotations, m)
	}
	return state, recorded, diags
}

// onCallTeamStartedAt chooses the started_at to store for a rotation.
//
// Waroom adjusts started_at to a shift handoff, so the value it returns usually differs from the
// configured one. To avoid a perpetual diff, the configured value is kept:
//   - after create / update (afterWrite), the planned value is stored as is;
//   - in read, the stored value is kept while the API still returns the started_at recorded at the
//     last write or read (recordedBefore). If it changed outside Terraform, or nothing was recorded
//     (e.g. after import), the API value is used.
func onCallTeamStartedAt(uuid string, api time.Time, prior *OnCallTeamRotationModel, recordedBefore map[string]time.Time, afterWrite bool) string {
	apiValue := api.Format(time.RFC3339)
	if prior == nil || prior.StartedAt.IsNull() || prior.StartedAt.IsUnknown() {
		return apiValue
	}
	if afterWrite {
		return prior.StartedAt.ValueString()
	}
	if prior.UUID.ValueString() != uuid {
		return apiValue
	}
	if last, ok := recordedBefore[uuid]; ok && last.Equal(api) {
		return prior.StartedAt.ValueString()
	}
	if t, err := time.Parse(time.RFC3339, prior.StartedAt.ValueString()); err == nil && t.Equal(api) {
		return prior.StartedAt.ValueString()
	}
	return apiValue
}

func getOnCallTeamStartedAtPrivate(ctx context.Context, p interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
}) (map[string]time.Time, diag.Diagnostics) {
	b, diags := p.GetKey(ctx, onCallTeamStartedAtPrivateKey)
	if diags.HasError() || len(b) == 0 {
		return nil, diags
	}
	var recorded map[string]time.Time
	if err := json.Unmarshal(b, &recorded); err != nil {
		// Treat broken private state as nothing recorded; the API values are used.
		tflog.Warn(ctx, "ignoring unreadable private state", map[string]interface{}{"key": onCallTeamStartedAtPrivateKey, "error": err.Error()})
		return nil, diags
	}
	return recorded, diags
}

func setOnCallTeamStartedAtPrivate(ctx context.Context, p interface {
	SetKey(context.Context, string, []byte) diag.Diagnostics
}, recorded map[string]time.Time) diag.Diagnostics {
	b, err := json.Marshal(recorded)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Internal Error", fmt.Sprintf("encoding private state, got error: %s", err))
		return diags
	}
	return p.SetKey(ctx, onCallTeamStartedAtPrivateKey, b)
}
