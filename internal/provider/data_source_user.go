package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/topotal/waroom-client-go/waroom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource                     = &UserDataSource{}
	_ datasource.DataSourceWithConfigure        = &UserDataSource{}
	_ datasource.DataSourceWithConfigValidators = &UserDataSource{}
)

// NewUserDataSource is a helper to simplify provider implementation.
func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

// UserDataSource defines the data source implementation.
type UserDataSource struct {
	client *waroom.Client
}

// UserDataSourceModel describes the data source data model.
type UserDataSourceModel struct {
	Id            types.String `tfsdk:"id"`
	Sub           types.String `tfsdk:"sub"`
	Nickname      types.String `tfsdk:"nickname"`
	SlackUserKey  types.String `tfsdk:"slack_user_key"`
	Picture       types.String `tfsdk:"picture"`
	Status        types.String `tfsdk:"status"`
	PhoneVerified types.Bool   `tfsdk:"phone_verified"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

// Metadata returns the data source type name.
func (d *UserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *UserDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	lookupValidators := []validator.String{stringvalidator.LengthAtLeast(1)}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a Waroom user. Look the user up by exactly one of `sub`, `nickname` or `slack_user_key`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "User ID. This is an alias for `sub`.",
				Computed:            true,
			},
			"sub": schema.StringAttribute{
				MarkdownDescription: "The user's subject identifier (e.g. `auth0|abc123`), which identifies a user in Waroom.",
				Optional:            true,
				Computed:            true,
				Validators:          lookupValidators,
			},
			"nickname": schema.StringAttribute{
				MarkdownDescription: "The user's nickname. Nicknames are unique in an organization.",
				Optional:            true,
				Computed:            true,
				Validators:          lookupValidators,
			},
			"slack_user_key": schema.StringAttribute{
				MarkdownDescription: "The ID of the Slack user linked to the user (e.g. `U0123ABCD`). Null when the user is not linked to Slack.",
				Optional:            true,
				Computed:            true,
				Validators:          lookupValidators,
			},
			"picture": schema.StringAttribute{
				MarkdownDescription: "The URL of the user's profile picture.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The user's status: `active`, `inactive`, `invited`, `invitation_pending`, `invitation_failed` or `deactivated`. " +
					"Users in every status can be looked up, so check that the user is `active` before adding them to an on-call rotation.",
				Computed: true,
			},
			"phone_verified": schema.BoolAttribute{
				MarkdownDescription: "Whether the user's phone number is verified. Only verified users can be added to on-call rotations.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The time the user was created, in RFC 3339 format.",
				Computed:            true,
			},
		},
	}
}

// ConfigValidators requires exactly one lookup attribute.
func (d *UserDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("sub"),
			path.MatchRoot("nickname"),
			path.MatchRoot("slack_user_key"),
		),
	}
}

// slackUserKey returns the key of the linked Slack user, or nil when the user is not linked to Slack.
func slackUserKey(null bool, key string) *string {
	if null {
		return nil
	}
	return &key
}

// fill sets the model from a user. It takes the values one by one because the get-user and
// list-users responses are different types with the same fields; adding a field to this
// signature makes both call sites fail to compile until they pass it.
func (m *UserDataSourceModel) fill(sub, nickname, picture, status string, slackUserKey *string, phoneVerified bool, createdAt time.Time) {
	m.Id = types.StringValue(sub)
	m.Sub = types.StringValue(sub)
	m.Nickname = types.StringValue(nickname)
	m.SlackUserKey = types.StringPointerValue(slackUserKey)
	m.Picture = types.StringValue(picture)
	m.Status = types.StringValue(status)
	m.PhoneVerified = types.BoolValue(phoneVerified)
	m.CreatedAt = types.StringValue(createdAt.Format(time.RFC3339))
}

// matchUsers returns the users whose nickname or linked Slack user ID equals the configured one.
// The list-users API already filters by these, but the result is checked again so that a filter
// the API ignored or matched loosely (e.g. ignoring case) never returns a different user.
func matchUsers(users []waroom.User, nickname, slackKey types.String) []waroom.User {
	var matches []waroom.User
	for _, u := range users {
		switch {
		case !nickname.IsNull():
			if u.Nickname == nickname.ValueString() {
				matches = append(matches, u)
			}
		case !slackKey.IsNull():
			if !u.SlackUser.Null && u.SlackUser.Value.Key == slackKey.ValueString() {
				matches = append(matches, u)
			}
		}
	}
	return matches
}

// Read refreshes the Terraform state with the latest data.
//
// A user looked up by `sub` is read with the get-user API. A user looked up by `nickname` or
// `slack_user_key` is read with the list-users API filtered by that attribute, which must match exactly one user.
func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Sub.IsNull() {
		user, err := d.client.Users.Get(ctx, waroom.GetUserParams{Sub: data.Sub.ValueString()})
		if err != nil {
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				resp.Diagnostics.AddError(
					"User Not Found",
					fmt.Sprintf("No Waroom user with sub %q was found.", data.Sub.ValueString()),
				)
				return
			}
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetUser, got error: %s", err))
			return
		}
		data.fill(user.Sub, user.Nickname, user.Picture, string(user.Status),
			slackUserKey(user.SlackUser.Null, user.SlackUser.Value.Key), user.PhoneVerified, user.CreatedAt)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	var (
		params     waroom.ListUsersParams
		lookupDesc string
	)
	if !data.Nickname.IsNull() {
		params.Nickname = waroom.NewOptString(data.Nickname.ValueString())
		lookupDesc = fmt.Sprintf("nickname %q", data.Nickname.ValueString())
	} else {
		params.SlackUserKey = waroom.NewOptString(data.SlackUserKey.ValueString())
		lookupDesc = fmt.Sprintf("slack_user_key %q", data.SlackUserKey.ValueString())
	}

	users, err := d.client.Users.List(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("ListUsers, got error: %s", err))
		return
	}

	matches := matchUsers(users, data.Nickname, data.SlackUserKey)
	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError("User Not Found", fmt.Sprintf("No Waroom user with %s was found.", lookupDesc))
		return
	case 1:
		u := matches[0]
		data.fill(u.Sub, u.Nickname, u.Picture, string(u.Status),
			slackUserKey(u.SlackUser.Null, u.SlackUser.Value.Key), u.PhoneVerified, u.CreatedAt)
	default:
		resp.Diagnostics.AddError(
			"Multiple Users Found",
			fmt.Sprintf("Multiple Waroom users with %s were found. Look the user up by `sub` instead.", lookupDesc),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
