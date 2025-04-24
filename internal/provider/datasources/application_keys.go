package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_                              datasource.DataSource              = &applicationKeysDataSource{}
	_                              datasource.DataSourceWithConfigure = &applicationKeysDataSource{}
	applicationKeyDataSourceSchema                                    = schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"workspace_id": schema.StringAttribute{
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
)

// helper function to simplify the provider implementation.
func NewApplicationKeysDataSource() datasource.DataSource {
	return &applicationKeysDataSource{}
}

// applicationKeysDataSource is the data source implementation.
type applicationKeysDataSource struct {
	client *api.Client
}

// applicationKeysDataSourceModel maps the data source schema data.
type applicationKeysDataSourceModel struct {
	ApplicationKeys []models.ApplicationKey `tfsdk:"application_keys"`
}

// Metadata returns the data source type name.
func (d *applicationKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_keys"
}

// Schema defines the schema for the data source.
func (d *applicationKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This data source lists application keys.

If the WarpStream provider is authenticated with an application key, this data source lists application keys in that key's workspace only.
If the WarpStream provider is authenticated with an account key, it lists application keys in all workspaces.
`,
		Attributes: map[string]schema.Attribute{
			"application_keys": schema.ListNestedAttribute{Computed: true, NestedObject: applicationKeyDataSourceSchema},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *applicationKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state applicationKeysDataSourceModel
	apiKeys, err := d.client.GetAPIKeys()
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read WarpStream Application Keys", err.Error())
		return
	}

	applicationKeys := filterApplicationKeys(apiKeys)

	// Map response body to model
	mapped := models.MapToApplicationKeys(&applicationKeys)
	state.ApplicationKeys = *mapped

	diags := resp.State.Set(ctx, &state)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func filterApplicationKeys(apiKeys []api.APIKey) []api.APIKey {
	var applicationKeys []api.APIKey

	for _, apiKey := range apiKeys {
		for _, grant := range apiKey.AccessGrants {
			if grant.PrincipalKind == api.PrincipalKindApplication || grant.PrincipalKind == api.PrincipalKindAny {
				applicationKeys = append(applicationKeys, apiKey)
				break
			}
		}
	}

	return applicationKeys
}

// Configure adds the provider configured client to the data source.
func (d *applicationKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
