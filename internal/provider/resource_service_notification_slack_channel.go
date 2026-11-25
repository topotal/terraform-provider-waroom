package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"

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
	_ resource.Resource                = &ServiceNotificationSlackChannelResource{}
	_ resource.ResourceWithConfigure   = &ServiceNotificationSlackChannelResource{}
	_ resource.ResourceWithImportState = &ServiceNotificationSlackChannelResource{}
)

// NewServiceNotificationSlackChannelResource is a helper to simplify provider implementation.
func NewServiceNotificationSlackChannelResource() resource.Resource {
	return &ServiceNotificationSlackChannelResource{}
}

// ServiceNotificationSlackChannelResource is the resource implementation.
type ServiceNotificationSlackChannelResource struct {
	client *waroom.Client
}

// ServiceNotificationSlackChannelResourceModel describes the resource data model.
type ServiceNotificationSlackChannelResourceModel struct {
	Id          types.String `tfsdk:"id"`
	ServiceName types.String `tfsdk:"service_name"`
	ChannelId   types.String `tfsdk:"channel_id"`
	ChannelName types.String `tfsdk:"channel_name"`
}

// Metadata returns the resource type name.
func (r *ServiceNotificationSlackChannelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_notification_slack_channel"
}

func (r *ServiceNotificationSlackChannelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *ServiceNotificationSlackChannelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Slack channel that a Waroom service sends incident notifications to.\n\n" +
			"~> **Note:** A service has at most one notification Slack channel, so manage at most one of this resource per service. " +
			"Creating this resource overwrites any channel already set on the service, " +
			"and destroying it removes the channel from the service.\n\n" +
			"The channel must exist in the Slack workspace connected to Waroom: " +
			"Waroom looks up its name through the Slack API and rejects an unknown channel ID.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Resource ID. This is an alias for `service_name`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_name": schema.StringAttribute{
				MarkdownDescription: "The name of the Waroom service. Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"channel_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the Slack channel to send notifications to (e.g. `C1234567890`).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Z0-9]+$`), "must be a Slack channel ID such as C1234567890"),
				},
			},
			"channel_name": schema.StringAttribute{
				MarkdownDescription: "The name of the Slack channel, looked up by Waroom from `channel_id`.",
				Computed:            true,
			},
		},
	}
}

// Create sets the notification Slack channel of the service and sets the initial Terraform state.
//
// The API has no create operation; the channel is upserted with PutNotificationSlackChannel.
func (r *ServiceNotificationSlackChannelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceNotificationSlackChannelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.put(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("PutNotificationSlackChannel, got error: %s", err))
		return
	}

	tflog.Info(ctx, "created service notification slack channel", map[string]interface{}{
		"id":           state.Id.ValueString(),
		"service_name": state.ServiceName.ValueString(),
		"channel_id":   state.ChannelId.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Read refreshes the Terraform state with the latest data.
//
// The API returns 404 both when no channel is set and when the service is missing; both are treated as absent.
func (r *ServiceNotificationSlackChannelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServiceNotificationSlackChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	channel, err := r.client.Services.GetNotificationSlackChannel(ctx, waroom.GetServiceNotificationSlackChannelParams{
		Name: data.ServiceName.ValueString(),
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetNotificationSlackChannel, got error: %s", err))
		return
	}

	data.Id = data.ServiceName
	data.ChannelId = types.StringValue(channel.Key)
	data.ChannelName = types.StringValue(channel.Name.Or(""))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update changes the notification Slack channel and sets the updated Terraform state on success.
func (r *ServiceNotificationSlackChannelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceNotificationSlackChannelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	state, err := r.put(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("PutNotificationSlackChannel, got error: %s", err))
		return
	}

	tflog.Info(ctx, "updated service notification slack channel", map[string]interface{}{
		"id":           state.Id.ValueString(),
		"service_name": state.ServiceName.ValueString(),
		"channel_id":   state.ChannelId.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete removes the notification Slack channel from the service and removes the Terraform state on success.
func (r *ServiceNotificationSlackChannelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServiceNotificationSlackChannelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Services.DeleteNotificationSlackChannel(ctx, waroom.DeleteServiceNotificationSlackChannelParams{
		Name: state.ServiceName.ValueString(),
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("DeleteNotificationSlackChannel, got error: %s", err))
		return
	}
}

// ImportState imports the notification Slack channel of an existing service into Terraform state.
//
// The import ID is the name of the service.
//
// Example:
//
//	terraform import waroom_service_notification_slack_channel.example my-service
func (r *ServiceNotificationSlackChannelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("service_name"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// put upserts the channel from the plan and returns the resulting state.
func (r *ServiceNotificationSlackChannelResource) put(ctx context.Context, plan ServiceNotificationSlackChannelResourceModel) (ServiceNotificationSlackChannelResourceModel, error) {
	updated, err := r.client.Services.PutNotificationSlackChannel(ctx,
		waroom.PutNotificationSlackChannelReq{Key: plan.ChannelId.ValueString()},
		waroom.PutServiceNotificationSlackChannelParams{Name: plan.ServiceName.ValueString()},
	)
	if err != nil {
		return ServiceNotificationSlackChannelResourceModel{}, err
	}

	return ServiceNotificationSlackChannelResourceModel{
		Id:          plan.ServiceName,
		ServiceName: plan.ServiceName,
		ChannelId:   types.StringValue(updated.Key),
		ChannelName: types.StringValue(updated.Name.Or("")),
	}, nil
}
