package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/topotal/waroom-client-go/waroom"

	waroomvalidator "github.com/topotal/terraform-provider-waroom/internal/validator"
)

// onCallTriggerListPerPage is the page size used when looking up a trigger by name (the API maximum).
const onCallTriggerListPerPage = 100

var (
	_ datasource.DataSource                     = &OnCallTriggerDataSource{}
	_ datasource.DataSourceWithConfigure        = &OnCallTriggerDataSource{}
	_ datasource.DataSourceWithConfigValidators = &OnCallTriggerDataSource{}
)

// NewOnCallTriggerDataSource is a helper to simplify provider implementation.
func NewOnCallTriggerDataSource() datasource.DataSource {
	return &OnCallTriggerDataSource{}
}

// OnCallTriggerDataSource is the data source implementation.
type OnCallTriggerDataSource struct {
	client *waroom.Client
}

// OnCallTriggerDataSourceModel describes the data source data model.
type OnCallTriggerDataSourceModel struct {
	Id                 types.String `tfsdk:"id"`
	UUID               types.String `tfsdk:"uuid"`
	Name               types.String `tfsdk:"name"`
	ServiceName        types.String `tfsdk:"service_name"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	EventType          types.String `tfsdk:"event_type"`
	SeverityFilters    types.List   `tfsdk:"severity_filters"`
	SourceFilters      types.List   `tfsdk:"source_filters"`
	RetryCount         types.Int64  `tfsdk:"retry_count"`
	EscalationPolicies types.List   `tfsdk:"escalation_policies"`
}

var onCallTriggerEscalationPolicyAttrTypes = map[string]attr.Type{
	"uuid": types.StringType,
	"name": types.StringType,
}

// onCallTrigger is the API representation shared by the Get and List responses, whose generated types differ.
type onCallTrigger struct {
	UUID               string
	Name               string
	ServiceName        string
	Enabled            bool
	EventType          string
	SeverityFilters    []string
	SourceFilters      []string
	RetryCount         int
	EscalationPolicies []onCallTriggerEscalationPolicy
}

type onCallTriggerEscalationPolicy struct {
	UUID string
	Name string
}

// Metadata returns the data source type name.
func (d *OnCallTriggerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_on_call_trigger"
}

// Configure adds the provider configured client to the data source.
func (d *OnCallTriggerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *OnCallTriggerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a Waroom on-call trigger, the condition under which an incident calls escalation policies.\n\n" +
			"Look up a trigger either by `uuid` or by `name`. When looking up by `name`, set `service_name` too " +
			"if triggers in different services share the same name.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the trigger. This is an alias for `uuid`.",
				Computed:            true,
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The UUID of the trigger to look up. Exactly one of `uuid` or `name` must be set.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{waroomvalidator.UUID()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the trigger to look up. Exactly one of `uuid` or `name` must be set.",
				Optional:            true,
				Computed:            true,
			},
			"service_name": schema.StringAttribute{
				MarkdownDescription: "The name of the service the trigger belongs to. When set together with `name`, the lookup is limited to this service. " +
					"Cannot be set together with `uuid`.",
				Optional: true,
				Computed: true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the trigger is enabled.",
				Computed:            true,
			},
			"event_type": schema.StringAttribute{
				MarkdownDescription: "The event that fires the trigger. Currently only `incident_created`.",
				Computed:            true,
			},
			"severity_filters": schema.ListAttribute{
				MarkdownDescription: "The incident severities the trigger applies to (`critical`, `high`, `low`, `info`, `unknown`).",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"source_filters": schema.ListAttribute{
				MarkdownDescription: "The incident sources the trigger applies to (`integration`, `slack`, `web`).",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"retry_count": schema.Int64Attribute{
				MarkdownDescription: "How many times the escalation restarts from the beginning when no one responds.",
				Computed:            true,
			},
			"escalation_policies": schema.ListNestedAttribute{
				MarkdownDescription: "The escalation policies the trigger calls.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the escalation policy.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the escalation policy.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

// ConfigValidators requires exactly one lookup key and forbids scoping a UUID lookup by service.
func (d *OnCallTriggerDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("uuid"), path.MatchRoot("name")),
		datasourcevalidator.Conflicting(path.MatchRoot("uuid"), path.MatchRoot("service_name")),
	}
}

// Read refreshes the Terraform state with the latest data.
//
// A trigger is fetched with the Get API when `uuid` is set. Otherwise it is looked up by name
// by paging through the List API, optionally scoped by `service_name`.
func (d *OnCallTriggerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OnCallTriggerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var trigger *onCallTrigger
	if !data.UUID.IsNull() {
		trigger = d.getByUUID(ctx, data.UUID.ValueString(), resp)
	} else {
		trigger = d.findByName(ctx, data.Name.ValueString(), data.ServiceName.ValueString(), resp)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "read on-call trigger", map[string]any{
		"uuid":         trigger.UUID,
		"name":         trigger.Name,
		"service_name": trigger.ServiceName,
	})

	resp.Diagnostics.Append(data.fromAPI(ctx, trigger)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *OnCallTriggerDataSource) getByUUID(ctx context.Context, uuid string, resp *datasource.ReadResponse) *onCallTrigger {
	got, err := d.client.OnCallTriggers.Get(ctx, waroom.GetOnCallTriggerParams{UUID: uuid})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"On-Call Trigger Not Found",
				fmt.Sprintf("No Waroom on-call trigger with UUID %q was found.", uuid),
			)
			return nil
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetOnCallTrigger, got error: %s", err))
		return nil
	}
	return onCallTriggerFromGet(got)
}

func (d *OnCallTriggerDataSource) findByName(ctx context.Context, name, serviceName string, resp *datasource.ReadResponse) *onCallTrigger {
	params := waroom.ListOnCallTriggersParams{PerPage: waroom.NewOptInt(onCallTriggerListPerPage)}
	if serviceName != "" {
		params.ServiceName = waroom.NewOptString(serviceName)
	}

	var triggers []onCallTrigger
	for page := 1; ; page++ {
		params.Page = waroom.NewOptInt(page)
		items, err := d.client.OnCallTriggers.List(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("ListOnCallTriggers, got error: %s", err))
			return nil
		}
		for i := range items {
			triggers = append(triggers, *onCallTriggerFromListItem(&items[i]))
		}
		if len(items) < onCallTriggerListPerPage {
			break
		}
	}

	matches := filterOnCallTriggersByName(triggers, name)
	switch len(matches) {
	case 1:
		return &matches[0]
	case 0:
		msg := fmt.Sprintf("No Waroom on-call trigger named %q was found.", name)
		if serviceName != "" {
			msg = fmt.Sprintf("No Waroom on-call trigger named %q was found in service %q.", name, serviceName)
		}
		resp.Diagnostics.AddError("On-Call Trigger Not Found", msg)
	default:
		resp.Diagnostics.AddError(
			"Multiple On-Call Triggers Found",
			fmt.Sprintf("%d Waroom on-call triggers are named %q. Set `service_name` or `uuid` to select one.", len(matches), name),
		)
	}
	return nil
}

// filterOnCallTriggersByName returns the triggers whose name equals name.
func filterOnCallTriggersByName(triggers []onCallTrigger, name string) []onCallTrigger {
	var matches []onCallTrigger
	for _, t := range triggers {
		if t.Name == name {
			matches = append(matches, t)
		}
	}
	return matches
}

func onCallTriggerFromGet(t *waroom.OnCallTrigger) *onCallTrigger {
	out := &onCallTrigger{
		UUID:        t.UUID,
		Name:        t.Name,
		ServiceName: t.Service.Name,
		Enabled:     t.Enabled,
		EventType:   string(t.EventType),
		RetryCount:  t.RetryCount,
	}
	out.SeverityFilters = make([]string, 0, len(t.SeverityFilters))
	for _, v := range t.SeverityFilters {
		out.SeverityFilters = append(out.SeverityFilters, string(v))
	}
	out.SourceFilters = make([]string, 0, len(t.SourceFilters))
	for _, v := range t.SourceFilters {
		out.SourceFilters = append(out.SourceFilters, string(v))
	}
	out.EscalationPolicies = make([]onCallTriggerEscalationPolicy, 0, len(t.EscalationPolicies))
	for _, p := range t.EscalationPolicies {
		out.EscalationPolicies = append(out.EscalationPolicies, onCallTriggerEscalationPolicy{UUID: p.UUID, Name: p.Name})
	}
	return out
}

func onCallTriggerFromListItem(t *waroom.OnCallTriggerListItem) *onCallTrigger {
	out := &onCallTrigger{
		UUID:        t.UUID,
		Name:        t.Name,
		ServiceName: t.Service.Name,
		Enabled:     t.Enabled,
		EventType:   string(t.EventType),
		RetryCount:  t.RetryCount,
	}
	out.SeverityFilters = make([]string, 0, len(t.SeverityFilters))
	for _, v := range t.SeverityFilters {
		out.SeverityFilters = append(out.SeverityFilters, string(v))
	}
	out.SourceFilters = make([]string, 0, len(t.SourceFilters))
	for _, v := range t.SourceFilters {
		out.SourceFilters = append(out.SourceFilters, string(v))
	}
	out.EscalationPolicies = make([]onCallTriggerEscalationPolicy, 0, len(t.EscalationPolicies))
	for _, p := range t.EscalationPolicies {
		out.EscalationPolicies = append(out.EscalationPolicies, onCallTriggerEscalationPolicy{UUID: p.UUID, Name: p.Name})
	}
	return out
}

// fromAPI copies the trigger into the model.
func (m *OnCallTriggerDataSourceModel) fromAPI(ctx context.Context, t *onCallTrigger) (diags diag.Diagnostics) {
	m.Id = types.StringValue(t.UUID)
	m.UUID = types.StringValue(t.UUID)
	m.Name = types.StringValue(t.Name)
	m.ServiceName = types.StringValue(t.ServiceName)
	m.Enabled = types.BoolValue(t.Enabled)
	m.EventType = types.StringValue(t.EventType)
	m.RetryCount = types.Int64Value(int64(t.RetryCount))

	var d diag.Diagnostics
	m.SeverityFilters, d = types.ListValueFrom(ctx, types.StringType, t.SeverityFilters)
	diags.Append(d...)
	m.SourceFilters, d = types.ListValueFrom(ctx, types.StringType, t.SourceFilters)
	diags.Append(d...)

	policies := make([]attr.Value, 0, len(t.EscalationPolicies))
	for _, p := range t.EscalationPolicies {
		obj, d := types.ObjectValue(onCallTriggerEscalationPolicyAttrTypes, map[string]attr.Value{
			"uuid": types.StringValue(p.UUID),
			"name": types.StringValue(p.Name),
		})
		diags.Append(d...)
		policies = append(policies, obj)
	}
	m.EscalationPolicies, d = types.ListValue(types.ObjectType{AttrTypes: onCallTriggerEscalationPolicyAttrTypes}, policies)
	diags.Append(d...)

	return diags
}
