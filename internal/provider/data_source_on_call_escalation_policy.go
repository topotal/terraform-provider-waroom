package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	waroom "github.com/topotal/waroom-client-go/waroom"
)

var (
	_ datasource.DataSource                     = &OnCallEscalationPolicyDataSource{}
	_ datasource.DataSourceWithConfigure        = &OnCallEscalationPolicyDataSource{}
	_ datasource.DataSourceWithConfigValidators = &OnCallEscalationPolicyDataSource{}
)

// onCallEscalationPolicyListPerPage is the page size used when looking up a policy by name.
// It is the maximum the API accepts.
const onCallEscalationPolicyListPerPage = 100

// NewOnCallEscalationPolicyDataSource is a helper to simplify provider implementation.
func NewOnCallEscalationPolicyDataSource() datasource.DataSource {
	return &OnCallEscalationPolicyDataSource{}
}

// OnCallEscalationPolicyDataSource is the data source implementation.
type OnCallEscalationPolicyDataSource struct {
	client *waroom.Client
}

// OnCallEscalationPolicyDataSourceModel describes the data source data model.
type OnCallEscalationPolicyDataSourceModel struct {
	Id            types.String                          `tfsdk:"id"`
	UUID          types.String                          `tfsdk:"uuid"`
	Name          types.String                          `tfsdk:"name"`
	Timezone      types.String                          `tfsdk:"timezone"`
	CallDepth     types.Int64                           `tfsdk:"call_depth"`
	CallTraversal types.String                          `tfsdk:"call_traversal"`
	Steps         []OnCallEscalationPolicyStepModel     `tfsdk:"steps"`
	TimeSlots     []OnCallEscalationPolicyTimeSlotModel `tfsdk:"time_slots"`
}

// OnCallEscalationPolicyStepModel describes a step of an escalation policy.
type OnCallEscalationPolicyStepModel struct {
	UUID             types.String                        `tfsdk:"uuid"`
	Position         types.Int64                         `tfsdk:"position"`
	StepType         types.String                        `tfsdk:"step_type"`
	SimultaneousCall types.Bool                          `tfsdk:"simultaneous_call"`
	Team             OnCallEscalationPolicyStepTeamModel `tfsdk:"team"`
}

// OnCallEscalationPolicyStepTeamModel describes the on-call team called in a step.
type OnCallEscalationPolicyStepTeamModel struct {
	UUID  types.String `tfsdk:"uuid"`
	Name  types.String `tfsdk:"name"`
	Color types.String `tfsdk:"color"`
}

// OnCallEscalationPolicyTimeSlotModel describes a time slot of an escalation policy.
type OnCallEscalationPolicyTimeSlotModel struct {
	UUID      types.String `tfsdk:"uuid"`
	DayOfWeek types.Int64  `tfsdk:"day_of_week"`
	StartTime types.String `tfsdk:"start_time"`
	EndTime   types.String `tfsdk:"end_time"`
}

// Metadata returns the data source type name.
func (d *OnCallEscalationPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_on_call_escalation_policy"
}

// Configure adds the provider configured client to the data source.
func (d *OnCallEscalationPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*waroom.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = client
}

// Schema defines the schema for the data source.
func (d *OnCallEscalationPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a Waroom on-call escalation policy. " +
			"Look it up by exactly one of `uuid` or `name`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Escalation policy ID. This is an alias for `uuid`.",
				Computed:            true,
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The UUID of the escalation policy to look up. Exactly one of `uuid` or `name` must be set.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the escalation policy to look up. Exactly one of `uuid` or `name` must be set. " +
					"It is an error if no policy or more than one policy has this name.",
				Optional: true,
				Computed: true,
			},
			"timezone": schema.StringAttribute{
				MarkdownDescription: "The timezone (IANA format, e.g. `Asia/Tokyo`) that `time_slots` are interpreted in.",
				Computed:            true,
			},
			"call_depth": schema.Int64Attribute{
				MarkdownDescription: "How many people, counted from the on-duty user, are called in each step's team. " +
					"`null` means unlimited (everyone in the team).",
				Computed: true,
			},
			"call_traversal": schema.StringAttribute{
				MarkdownDescription: "The order in which steps and people are called: `depth_first` or `breadth_first`.",
				Computed:            true,
			},
			"steps": schema.ListNestedAttribute{
				MarkdownDescription: "The escalation steps, in escalation order (ascending `position`).",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the step.",
							Computed:            true,
						},
						"position": schema.Int64Attribute{
							MarkdownDescription: "The escalation order of the step, starting from 0.",
							Computed:            true,
						},
						"step_type": schema.StringAttribute{
							MarkdownDescription: "The type of the step: `primary` or `fallback`.",
							Computed:            true,
						},
						"simultaneous_call": schema.BoolAttribute{
							MarkdownDescription: "Whether all participants of the team's active rotation are called at the same time in this step.",
							Computed:            true,
						},
						"team": schema.SingleNestedAttribute{
							MarkdownDescription: "The on-call team called in this step.",
							Computed:            true,
							Attributes: map[string]schema.Attribute{
								"uuid": schema.StringAttribute{
									MarkdownDescription: "The UUID of the on-call team.",
									Computed:            true,
								},
								"name": schema.StringAttribute{
									MarkdownDescription: "The name of the on-call team.",
									Computed:            true,
								},
								"color": schema.StringAttribute{
									MarkdownDescription: "The color of the on-call team in hex format without the `#` prefix.",
									Computed:            true,
								},
							},
						},
					},
				},
			},
			"time_slots": schema.ListNestedAttribute{
				MarkdownDescription: "The days of the week and times when the escalation policy is active, ordered by `day_of_week` and `start_time`.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the time slot.",
							Computed:            true,
						},
						"day_of_week": schema.Int64Attribute{
							MarkdownDescription: "The day of the week (0 is Sunday, 6 is Saturday).",
							Computed:            true,
						},
						"start_time": schema.StringAttribute{
							MarkdownDescription: "The start time (`HH:MM`) in the policy's `timezone`.",
							Computed:            true,
						},
						"end_time": schema.StringAttribute{
							MarkdownDescription: "The end time (`HH:MM`) in the policy's `timezone`. " +
								"If it is earlier than `start_time`, the slot continues into the next day.",
							Computed: true,
						},
					},
				},
			},
		},
	}
}

// ConfigValidators requires exactly one of uuid or name.
func (d *OnCallEscalationPolicyDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("uuid"),
			path.MatchRoot("name"),
		),
	}
}

// Read refreshes the Terraform state with the latest data.
//
// When looked up by name, the List API (which has no name filter) is paged through to find the UUID,
// and the policy is then read with the Get API.
func (d *OnCallEscalationPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OnCallEscalationPolicyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := data.UUID.ValueString()
	if data.UUID.IsNull() {
		name := data.Name.ValueString()
		uuids, err := d.findOnCallEscalationPolicyUUIDsByName(ctx, name)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("ListOnCallEscalationPolicies, got error: %s", err))
			return
		}
		switch len(uuids) {
		case 0:
			resp.Diagnostics.AddError(
				"On-Call Escalation Policy Not Found",
				fmt.Sprintf("No Waroom on-call escalation policy named %q was found.", name),
			)
			return
		case 1:
			uuid = uuids[0]
		default:
			resp.Diagnostics.AddError(
				"Multiple On-Call Escalation Policies Found",
				fmt.Sprintf("%d Waroom on-call escalation policies are named %q. Look it up by uuid instead.", len(uuids), name),
			)
			return
		}
	}

	policy, err := d.client.OnCallEscalationPolicies.Get(ctx, waroom.GetOnCallEscalationPolicyParams{UUID: uuid})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"On-Call Escalation Policy Not Found",
				fmt.Sprintf("No Waroom on-call escalation policy with UUID %q was found.", uuid),
			)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetOnCallEscalationPolicy, got error: %s", err))
		return
	}

	data = newOnCallEscalationPolicyDataSourceModel(policy)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// findOnCallEscalationPolicyUUIDsByName pages through the List API and returns the UUIDs of every policy with the given name.
func (d *OnCallEscalationPolicyDataSource) findOnCallEscalationPolicyUUIDsByName(ctx context.Context, name string) ([]string, error) {
	var uuids []string
	for page := 1; ; page++ {
		policies, err := d.client.OnCallEscalationPolicies.List(ctx, waroom.ListOnCallEscalationPoliciesParams{
			Page:    waroom.NewOptInt(page),
			PerPage: waroom.NewOptInt(onCallEscalationPolicyListPerPage),
		})
		if err != nil {
			return nil, err
		}
		for _, policy := range policies {
			if policy.Name == name {
				uuids = append(uuids, policy.UUID)
			}
		}
		if len(policies) < onCallEscalationPolicyListPerPage {
			return uuids, nil
		}
	}
}

// newOnCallEscalationPolicyDataSourceModel converts an API escalation policy into the data source model.
func newOnCallEscalationPolicyDataSourceModel(policy *waroom.OnCallEscalationPolicy) OnCallEscalationPolicyDataSourceModel {
	data := OnCallEscalationPolicyDataSourceModel{
		Id:            types.StringValue(policy.UUID),
		UUID:          types.StringValue(policy.UUID),
		Name:          types.StringValue(policy.Name),
		Timezone:      types.StringValue(policy.Timezone),
		CallDepth:     types.Int64Null(),
		CallTraversal: types.StringValue(string(policy.CallTraversal)),
		Steps:         make([]OnCallEscalationPolicyStepModel, 0, len(policy.Steps)),
		TimeSlots:     make([]OnCallEscalationPolicyTimeSlotModel, 0, len(policy.TimeSlots)),
	}
	if depth, ok := policy.CallDepth.Get(); ok {
		data.CallDepth = types.Int64Value(int64(depth))
	}
	for _, step := range policy.Steps {
		data.Steps = append(data.Steps, OnCallEscalationPolicyStepModel{
			UUID:             types.StringValue(step.UUID),
			Position:         types.Int64Value(int64(step.Position)),
			StepType:         types.StringValue(string(step.StepType)),
			SimultaneousCall: types.BoolValue(step.SimultaneousCall),
			Team: OnCallEscalationPolicyStepTeamModel{
				UUID:  types.StringValue(step.Team.UUID),
				Name:  types.StringValue(step.Team.Name),
				Color: types.StringValue(step.Team.Color),
			},
		})
	}
	for _, slot := range policy.TimeSlots {
		data.TimeSlots = append(data.TimeSlots, OnCallEscalationPolicyTimeSlotModel{
			UUID:      types.StringValue(slot.UUID),
			DayOfWeek: types.Int64Value(int64(slot.DayOfWeek)),
			StartTime: types.StringValue(slot.StartTime),
			EndTime:   types.StringValue(slot.EndTime),
		})
	}
	return data
}
