package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &workspaceDataSource{}
	_ datasource.DataSourceWithConfigure = &workspaceDataSource{}
)

// helper function to simplify the provider implementation.
func NewWorkspaceDataSource() datasource.DataSource {
	return &workspaceDataSource{}
}

// workspaceDataSource is the data source implementation.
type workspaceDataSource struct {
	client *api.Client
}

// Metadata returns the data source type name.
func (d *workspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

// Schema defines the schema for the data source.
func (d *workspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This data source reads individual Workspaces and their respective application keys.

The WarpStream provider must be authenticated with an account key to read this data source.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
			"application_keys": schema.ListNestedAttribute{
				Computed:     true,
				NestedObject: applicationKeyDataSourceSchema,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data models.WorkspaceDataSource
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := data.ID.ValueString()
	workspace, err := d.client.GetWorkspace(workspaceID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read WarpStream Workspace", err.Error())
		return
	}

	data.Name = types.StringValue(workspace.Name)
	data.CreatedAt = types.StringValue(workspace.CreatedAt)

	apiKeys, err := d.client.GetAPIKeys()
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read WarpStream Application Keys", err.Error())
		return
	}

	applicationKeys := filterForWorkspace(filterApplicationKeys(apiKeys), workspaceID)

	mapped := models.MapToApplicationKeys(&applicationKeys)
	data.ApplicationKeys = *mapped

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func filterForWorkspace(keys []api.APIKey, workspaceID string) []api.APIKey {
	var filtered []api.APIKey

	for _, key := range keys {
		for _, grant := range key.AccessGrants {
			if grant.WorkspaceID == workspaceID {
				filtered = append(filtered, key)
				break
			}
		}
	}

	return filtered
}

// Configure adds the provider configured client to the data source.
func (d *workspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}
