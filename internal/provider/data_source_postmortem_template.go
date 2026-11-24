package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/topotal/waroom-client-go/waroom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource                     = &PostmortemTemplateDataSource{}
	_ datasource.DataSourceWithConfigure        = &PostmortemTemplateDataSource{}
	_ datasource.DataSourceWithConfigValidators = &PostmortemTemplateDataSource{}
)

// postmortemTemplatesPerPage is the page size used when listing postmortem templates to look one up by title.
const postmortemTemplatesPerPage = 100

// NewPostmortemTemplateDataSource is a helper to simplify provider implementation.
func NewPostmortemTemplateDataSource() datasource.DataSource {
	return &PostmortemTemplateDataSource{}
}

// PostmortemTemplateDataSource defines the data source implementation.
type PostmortemTemplateDataSource struct {
	client *waroom.Client
}

// PostmortemTemplateDataSourceModel describes the data source data model.
type PostmortemTemplateDataSourceModel struct {
	Id        types.String `tfsdk:"id"`
	UUID      types.String `tfsdk:"uuid"`
	Title     types.String `tfsdk:"title"`
	Blob      types.String `tfsdk:"blob"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

// Metadata returns the data source type name.
func (d *PostmortemTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_postmortem_template"
}

// Schema defines the schema for the data source.
func (d *PostmortemTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a Waroom postmortem template. " +
			"Look the template up by exactly one of `uuid` or `title`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Postmortem template ID. This is an alias for `uuid`.",
				Computed:            true,
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The unique identifier (UUID) of the postmortem template. Exactly one of `uuid` or `title` must be set.",
				Optional:            true,
				Computed:            true,
			},
			"title": schema.StringAttribute{
				MarkdownDescription: "The title of the postmortem template. Exactly one of `uuid` or `title` must be set. " +
					"When looking up by title, the title must match exactly one template.",
				Optional: true,
				Computed: true,
			},
			"blob": schema.StringAttribute{
				MarkdownDescription: "The content of the postmortem template, in Markdown.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp (RFC 3339) when the postmortem template was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp (RFC 3339) when the postmortem template was last updated.",
				Computed:            true,
			},
		},
	}
}

// ConfigValidators requires exactly one of uuid or title.
func (d *PostmortemTemplateDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("uuid"),
			path.MatchRoot("title"),
		),
	}
}

func (d *PostmortemTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Read refreshes the Terraform state with the latest data.
//
// With uuid, the template is fetched directly. With title, all templates are listed page by page and
// the title must match exactly one of them.
func (d *PostmortemTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PostmortemTemplateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.UUID.IsNull() {
		id, err := uuid.Parse(data.UUID.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("uuid"), "Invalid UUID", fmt.Sprintf("%q is not a valid UUID: %s", data.UUID.ValueString(), err))
			return
		}

		tmpl, err := d.client.PostmortemTemplates.Get(ctx, waroom.GetPostmortemTemplateByUuidParams{UUID: id})
		if err != nil {
			var apiErr *waroom.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				resp.Diagnostics.AddError(
					"Postmortem Template Not Found",
					fmt.Sprintf("No Waroom postmortem template with UUID %q was found.", data.UUID.ValueString()),
				)
				return
			}
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetPostmortemTemplate, got error: %s", err))
			return
		}

		data.setFromAPI(waroom.PostmortemTemplateListItem(*tmpl))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	title := data.Title.ValueString()
	var matches []waroom.PostmortemTemplateListItem
	for page := 1; ; page++ {
		params := waroom.ListPostmortemTemplatesParams{}
		params.Page.SetTo(page)
		params.PerPage.SetTo(postmortemTemplatesPerPage)

		templates, err := d.client.PostmortemTemplates.List(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("ListPostmortemTemplates, got error: %s", err))
			return
		}
		if len(templates) == 0 {
			break
		}

		for _, tmpl := range templates {
			if v, ok := tmpl.Title.Get(); ok && v == title {
				matches = append(matches, tmpl)
			}
		}
	}

	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError(
			"Postmortem Template Not Found",
			fmt.Sprintf("No Waroom postmortem template titled %q was found.", title),
		)
		return
	case 1:
		data.setFromAPI(matches[0])
	default:
		resp.Diagnostics.AddError(
			"Multiple Postmortem Templates Found",
			fmt.Sprintf("%d Waroom postmortem templates are titled %q. Use uuid to choose one.", len(matches), title),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (m *PostmortemTemplateDataSourceModel) setFromAPI(tmpl waroom.PostmortemTemplateListItem) {
	m.Id = types.StringValue(tmpl.UUID.String())
	m.UUID = types.StringValue(tmpl.UUID.String())
	if v, ok := tmpl.Title.Get(); ok {
		m.Title = types.StringValue(v)
	} else {
		m.Title = types.StringNull()
	}
	m.Blob = types.StringValue(tmpl.Blob)
	m.CreatedAt = types.StringValue(tmpl.CreatedAt.Format(time.RFC3339))
	m.UpdatedAt = types.StringValue(tmpl.UpdatedAt.Format(time.RFC3339))
}
