package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/topotal/waroom-client-go/waroom"
)

// onCallTeamListPerPage is the page size used when looking up an on-call team by name (the API maximum).
const onCallTeamListPerPage = 100

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource                     = &OnCallTeamDataSource{}
	_ datasource.DataSourceWithConfigure        = &OnCallTeamDataSource{}
	_ datasource.DataSourceWithConfigValidators = &OnCallTeamDataSource{}
)

// NewOnCallTeamDataSource is a helper to simplify provider implementation.
func NewOnCallTeamDataSource() datasource.DataSource {
	return &OnCallTeamDataSource{}
}

// OnCallTeamDataSource defines the data source implementation.
type OnCallTeamDataSource struct {
	client *waroom.Client
}

// OnCallTeamDataSourceModel describes the data source data model.
type OnCallTeamDataSourceModel struct {
	Id    types.String `tfsdk:"id"`
	UUID  types.String `tfsdk:"uuid"`
	Name  types.String `tfsdk:"name"`
	Color types.String `tfsdk:"color"`
}

// Metadata returns the data source type name.
func (d *OnCallTeamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_on_call_team"
}

func (d *OnCallTeamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *OnCallTeamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a Waroom on-call team. Look it up by either `uuid` or `name`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "On-call team ID. This is an alias for `uuid`.",
				Computed:            true,
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The UUID of the on-call team to look up. Exactly one of `uuid` or `name` must be set.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the on-call team to look up. Exactly one of `uuid` or `name` must be set. " +
					"It is an error if no team or more than one team has this name.",
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"color": schema.StringAttribute{
				MarkdownDescription: "The color of the on-call team, as a 6-digit hex code without `#` (e.g. `FF0000`).",
				Computed:            true,
			},
		},
	}
}

// ConfigValidators requires exactly one of uuid or name.
func (d *OnCallTeamDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("uuid"),
			path.MatchRoot("name"),
		),
	}
}

// Read refreshes the Terraform state with the latest data.
//
// A team is fetched with Get when `uuid` is set. When `name` is set, the List API has no
// name filter, so the teams are paged through and matched by name.
func (d *OnCallTeamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OnCallTeamDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.UUID.IsNull() {
		team, err := d.client.OnCallTeams.Get(ctx, waroom.GetOnCallTeamParams{UUID: data.UUID.ValueString()})
		if err != nil {
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				resp.Diagnostics.AddError(
					"On-Call Team Not Found",
					fmt.Sprintf("No Waroom on-call team with UUID %q was found.", data.UUID.ValueString()),
				)
				return
			}
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetOnCallTeam, got error: %s", err))
			return
		}
		setOnCallTeamDataSourceModel(&data, team.UUID, team.Name, team.Color)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	name := data.Name.ValueString()
	var matches []waroom.OnCallTeamListItem
	for page := 1; ; page++ {
		teams, err := d.client.OnCallTeams.List(ctx, waroom.ListOnCallTeamsParams{
			Page:    waroom.NewOptInt(page),
			PerPage: waroom.NewOptInt(onCallTeamListPerPage),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("ListOnCallTeams, got error: %s", err))
			return
		}
		for _, team := range teams {
			if team.Name == name {
				matches = append(matches, team)
			}
		}
		if len(teams) < onCallTeamListPerPage {
			break
		}
	}

	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError(
			"On-Call Team Not Found",
			fmt.Sprintf("No Waroom on-call team named %q was found.", name),
		)
		return
	case 1:
		setOnCallTeamDataSourceModel(&data, matches[0].UUID, matches[0].Name, matches[0].Color)
	default:
		resp.Diagnostics.AddError(
			"Multiple On-Call Teams Found",
			fmt.Sprintf("%d Waroom on-call teams named %q were found. Look the team up by `uuid` instead.", len(matches), name),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func setOnCallTeamDataSourceModel(data *OnCallTeamDataSourceModel, uuid, name, color string) {
	data.Id = types.StringValue(uuid)
	data.UUID = types.StringValue(uuid)
	data.Name = types.StringValue(name)
	data.Color = types.StringValue(color)
}
