package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/topotal/waroom-client-go/waroom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &RunbookDataSource{}
	_ datasource.DataSourceWithConfigure = &RunbookDataSource{}
)

// NewRunbookDataSource is a helper to simplify provider implementation.
func NewRunbookDataSource() datasource.DataSource {
	return &RunbookDataSource{}
}

// RunbookDataSource defines the data source implementation.
type RunbookDataSource struct {
	client *waroom.Client
}

// RunbookDataSourceModel describes the data source data model.
type RunbookDataSourceModel struct {
	Id        types.String `tfsdk:"id"`
	Namespace types.String `tfsdk:"namespace"`
	Blob      types.String `tfsdk:"blob"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

// Metadata returns the data source type name.
func (d *RunbookDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runbook"
}

func (d *RunbookDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *RunbookDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a Waroom runbook. A runbook is identified by its path-style namespace " +
			"(e.g. `/default`, `/api-server/recovery`).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Runbook ID. This is the namespace as returned by Waroom, with a leading slash (e.g. `/api-server/recovery`).",
				Computed:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "The namespace of the runbook to look up, e.g. `/api-server/recovery`. The leading slash is optional. " +
					"It may contain only ASCII letters, digits, `_`, `-` and `/`, and is at most 255 characters including the leading slash. " +
					"The root namespace `/` is not supported.",
				Required:   true,
				Validators: runbookNamespaceValidators(),
			},
			"blob": schema.StringAttribute{
				MarkdownDescription: "Markdown content of the runbook.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The time the runbook was created, in RFC 3339 format.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "The time the runbook was last updated, in RFC 3339 format.",
				Computed:            true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *RunbookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RunbookDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	runbook, err := d.client.Runbooks.Get(ctx, waroom.GetRunbookParams{
		Namespace: runbookNamespaceParam(data.Namespace.ValueString()),
	})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Runbook Not Found",
				fmt.Sprintf("No Waroom runbook with namespace %q was found.", data.Namespace.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("GetRunbook, got error: %s", err))
		return
	}

	// Keep namespace as configured; id holds the canonical namespace returned by the API.
	data.Id = types.StringValue(runbook.Namespace)
	data.Blob = types.StringValue(runbook.Blob)
	data.CreatedAt = runbookTimeValue(runbook.CreatedAt.Value, runbook.CreatedAt.Set)
	data.UpdatedAt = runbookTimeValue(runbook.UpdatedAt.Value, runbook.UpdatedAt.Set)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// runbookNamespaceValidators mirrors the API's namespace rule (`\A/[\w\-/]*\z`, at most 255 characters),
// with the leading slash made optional. The root namespace "/" (and anything starting with "//") is rejected:
// with the slash stripped its Get path would be empty, which the API answers with the runbook list instead.
func runbookNamespaceValidators() []validator.String {
	return []validator.String{
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^/?[A-Za-z0-9_/-]*$`),
			"may contain only ASCII letters, digits, `_`, `-` and `/`, with an optional leading slash",
		),
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^/?[^/]`),
			"must not be empty or the root namespace `/`; use a namespace such as `/default` or `api-server/recovery`",
		),
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^(/.{0,254}|[^/].{0,253})$`),
			"must be at most 255 characters including the leading slash",
		),
	}
}

// runbookNamespaceWithSlashValidators is runbookNamespaceValidators with the leading slash required,
// for attributes compared with the namespace as the API stores it (waroom_runbook's namespace, and
// waroom_service_runbook_rule's runbook_namespace, which the API looks up as given).
func runbookNamespaceWithSlashValidators() []validator.String {
	return append(runbookNamespaceValidators(),
		stringvalidator.RegexMatches(regexp.MustCompile(`^/`), "must start with `/` (e.g. `/default`)"),
	)
}

// runbookNamespaceParam converts a namespace to the form the runbook API expects in its path:
// without the leading slash (e.g. "/api-server/recovery" -> "api-server/recovery").
func runbookNamespaceParam(namespace string) string {
	return strings.TrimPrefix(namespace, "/")
}

// runbookTimeValue converts an optional API timestamp to an RFC 3339 string, or null when unset.
func runbookTimeValue(t time.Time, set bool) types.String {
	if !set {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
