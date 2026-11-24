package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	waroom "github.com/topotal/waroom-client-go/waroom"
)

var (
	_ datasource.DataSource              = &ServiceWebhookConfigurationDataSource{}
	_ datasource.DataSourceWithConfigure = &ServiceWebhookConfigurationDataSource{}
)

// webhookConfigurationsPerPage is the page size used when looking up a webhook configuration by name
// (the API maximum).
const webhookConfigurationsPerPage = 100

// webhookSubscriptionEventAttrTypes are the attribute types of an element of webhook_subscription_events.
var webhookSubscriptionEventAttrTypes = map[string]attr.Type{
	"uuid":          types.StringType,
	"event_type":    types.StringType,
	"event_version": types.StringType,
}

// NewServiceWebhookConfigurationDataSource is a helper to simplify provider implementation.
func NewServiceWebhookConfigurationDataSource() datasource.DataSource {
	return &ServiceWebhookConfigurationDataSource{}
}

// ServiceWebhookConfigurationDataSource is the data source implementation.
type ServiceWebhookConfigurationDataSource struct {
	client *waroom.Client
}

// ServiceWebhookConfigurationDataSourceModel describes the data source data model.
type ServiceWebhookConfigurationDataSourceModel struct {
	Id                        types.String `tfsdk:"id"`
	ServiceName               types.String `tfsdk:"service_name"`
	UUID                      types.String `tfsdk:"uuid"`
	Name                      types.String `tfsdk:"name"`
	URL                       types.String `tfsdk:"url"`
	State                     types.String `tfsdk:"state"`
	WebhookSubscriptionEvents types.List   `tfsdk:"webhook_subscription_events"`
	CreatedAt                 types.String `tfsdk:"created_at"`
	UpdatedAt                 types.String `tfsdk:"updated_at"`
}

// Metadata returns the data source type name.
func (d *ServiceWebhookConfigurationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_webhook_configuration"
}

// Configure adds the provider configured client to the data source.
func (d *ServiceWebhookConfigurationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *ServiceWebhookConfigurationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	lookup := path.Expressions{path.MatchRoot("uuid"), path.MatchRoot("name")}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a Waroom webhook configuration of a service. " +
			"Look it up by `uuid` or by `name`; exactly one of them must be set.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Webhook configuration ID. This is an alias for `uuid`.",
				Computed:            true,
			},
			"service_name": schema.StringAttribute{
				MarkdownDescription: "The name of the Waroom service the webhook configuration belongs to.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The UUID of the webhook configuration to look up. Exactly one of `uuid` and `name` must be set.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(lookup...),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the webhook configuration to look up. Exactly one of `uuid` and `name` must be set. " +
					"It is an error if more than one webhook configuration in the service has this name.",
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(lookup...),
				},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The URL that webhook events are sent to.",
				Computed:            true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "The state of the webhook configuration: `unverified`, `verified` or `archived`.",
				Computed:            true,
			},
			"webhook_subscription_events": schema.ListNestedAttribute{
				MarkdownDescription: "The events the webhook configuration subscribes to.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							MarkdownDescription: "The UUID of the subscription.",
							Computed:            true,
						},
						"event_type": schema.StringAttribute{
							MarkdownDescription: "The event type, e.g. `incident_created`, `incident_updated`, `runbook_executed`, " +
								"`runbook_skipped`, `incident_slack_channel_created` or `test_ping`.",
							Computed: true,
						},
						"event_version": schema.StringAttribute{
							MarkdownDescription: "The version of the event payload, e.g. `v1`.",
							Computed:            true,
						},
					},
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The time the webhook configuration was created, in RFC 3339 format.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The time the webhook configuration was last updated, in RFC 3339 format.",
				Computed:            true,
			},
		},
	}
}

// Read refreshes Terraform state with the latest data.
//
// With `uuid` the webhook configuration is fetched directly. With `name` it is looked up in the
// list of the service's webhook configurations first, and then fetched by its UUID, since only
// the Get API returns the subscribed events.
func (d *ServiceWebhookConfigurationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ServiceWebhookConfigurationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceName := data.ServiceName.ValueString()

	var id uuid.UUID
	if !data.UUID.IsNull() {
		parsed, err := uuid.Parse(data.UUID.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("uuid"), "Invalid UUID", fmt.Sprintf("%q is not a valid UUID: %s", data.UUID.ValueString(), err))
			return
		}
		id = parsed
	} else {
		found, diags := d.findByName(ctx, serviceName, data.Name.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		id = found
	}

	webhookConfiguration, err := d.client.WebhookConfigurations.Get(ctx, waroom.GetServiceWebhookConfigurationParams{
		Name: serviceName,
		UUID: id,
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Webhook Configuration Not Found",
				fmt.Sprintf("No webhook configuration with UUID %q was found in the Waroom service %q.", id, serviceName),
			)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetServiceWebhookConfiguration, got error: %s", err))
		return
	}

	model, diags := webhookConfigurationToModel(serviceName, webhookConfiguration)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "read service_webhook_configuration data source", map[string]any{
		"service_name": serviceName,
		"uuid":         model.UUID.ValueString(),
		"name":         model.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// findByName returns the UUID of the only webhook configuration named name in the service.
func (d *ServiceWebhookConfigurationDataSource) findByName(ctx context.Context, serviceName, name string) (uuid.UUID, diag.Diagnostics) {
	var diags diag.Diagnostics
	var matches []uuid.UUID

	for page := 1; ; page++ {
		items, err := d.client.WebhookConfigurations.List(ctx, waroom.ListServiceWebhookConfigurationsParams{
			Name:    serviceName,
			Page:    waroom.NewOptInt(page),
			PerPage: waroom.NewOptInt(webhookConfigurationsPerPage),
		})
		if err != nil {
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				diags.AddError("Service Not Found", fmt.Sprintf("No Waroom service named %q was found.", serviceName))
				return uuid.Nil, diags
			}
			diags.AddError("Client Error", fmt.Sprintf("ListServiceWebhookConfigurations, got error: %s", err))
			return uuid.Nil, diags
		}

		for _, item := range items {
			if item.Name == name {
				matches = append(matches, item.UUID)
			}
		}

		if len(items) < webhookConfigurationsPerPage {
			break
		}
	}

	switch len(matches) {
	case 0:
		diags.AddError(
			"Webhook Configuration Not Found",
			fmt.Sprintf("No webhook configuration named %q was found in the Waroom service %q.", name, serviceName),
		)
		return uuid.Nil, diags
	case 1:
		return matches[0], diags
	default:
		diags.AddError(
			"Multiple Webhook Configurations Found",
			fmt.Sprintf("%d webhook configurations named %q were found in the Waroom service %q. Look it up by uuid instead.", len(matches), name, serviceName),
		)
		return uuid.Nil, diags
	}
}

// webhookConfigurationToModel converts an API webhook configuration into the data source model.
func webhookConfigurationToModel(serviceName string, w *waroom.WebhookConfiguration) (ServiceWebhookConfigurationDataSourceModel, diag.Diagnostics) {
	events := make([]attr.Value, 0, len(w.WebhookSubscriptionEvents))
	for _, e := range w.WebhookSubscriptionEvents {
		events = append(events, types.ObjectValueMust(webhookSubscriptionEventAttrTypes, map[string]attr.Value{
			"uuid":          types.StringValue(e.UUID.String()),
			"event_type":    types.StringValue(string(e.EventType)),
			"event_version": types.StringValue(string(e.EventVersion)),
		}))
	}

	eventList, diags := types.ListValue(types.ObjectType{AttrTypes: webhookSubscriptionEventAttrTypes}, events)

	return ServiceWebhookConfigurationDataSourceModel{
		Id:                        types.StringValue(w.UUID.String()),
		ServiceName:               types.StringValue(serviceName),
		UUID:                      types.StringValue(w.UUID.String()),
		Name:                      types.StringValue(w.Name),
		URL:                       types.StringValue(w.URL.String()),
		State:                     types.StringValue(string(w.State)),
		WebhookSubscriptionEvents: eventList,
		CreatedAt:                 types.StringValue(w.CreatedAt.Format(time.RFC3339)),
		UpdatedAt:                 types.StringValue(w.UpdatedAt.Format(time.RFC3339)),
	}, diags
}
