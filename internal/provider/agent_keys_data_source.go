package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &agentKeysDataSource{}
	_ datasource.DataSourceWithConfigure = &agentKeysDataSource{}
)

// helper function to simplify the provider implementation.
func NewAgentKeysDataSource() datasource.DataSource {
	return &agentKeysDataSource{}
}

// agentKeysDataSource is the data source implementation.
type agentKeysDataSource struct {
	client *api.Client
}

// agentKeysDataSourceModel maps the data source schema data.
type agentKeysDataSourceModel struct {
	AgentKeys []agentKeyModel `tfsdk:"agent_keys"`
}

// Metadata returns the data source type name.
func (d *agentKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_keys"
}

// Schema defines the schema for the data source.
func (d *agentKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This data source lists agent keys.

The WarpStream provider must be authenticated with an application key to read this data source.
`,
		Attributes: map[string]schema.Attribute{
			"agent_keys": schema.ListNestedAttribute{Computed: true, NestedObject: agentKeyDataSourceSchema},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *agentKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state agentKeysDataSourceModel
	apiKeys, err := d.client.GetAPIKeys()
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read WarpStream Agent Keys", err.Error())
		return
	}

	agentKeys := filterAgentKeys(apiKeys)

	// Map response body to model
	mapped, ok := mapToAgentKeyModels(&agentKeys, &resp.Diagnostics)
	if !ok {
		return // Diagnostics handled by helper.
	}
	state.AgentKeys = *mapped

	diags := resp.State.Set(ctx, &state)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func filterAgentKeys(apiKeys []api.APIKey) []api.APIKey {
	var agentKeys []api.APIKey

	for _, apiKey := range apiKeys {
		for _, grant := range apiKey.AccessGrants {
			if grant.PrincipalKind == api.PrincipalKindAgent &&
				grant.ResourceKind == api.ResourceKindVirtualCluster {
				agentKeys = append(agentKeys, apiKey)
				break
			}
		}
	}

	return agentKeys
}

// Configure adds the provider configured client to the data source.
func (d *agentKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
