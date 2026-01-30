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
	_ datasource.DataSource              = &ssoConfigurationDataSource{}
	_ datasource.DataSourceWithConfigure = &ssoConfigurationDataSource{}
)

// helper function to simplify the provider implementation.
func NewSSOConfigurationDataSource() datasource.DataSource {
	return &ssoConfigurationDataSource{}
}

// workspaceDataSource is the data source implementation.
type ssoConfigurationDataSource struct {
	client *api.Client
}

// Metadata returns the data source type name.
func (d *ssoConfigurationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_configuration"
}

// Schema defines the schema for the data source.
func (d *ssoConfigurationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This data source reads the tenant SSO Configuration.

The WarpStream provider must be authenticated with an account key to read this data source.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"sso_identifier": schema.StringAttribute{
				Computed: true,
			},
			"saml_url": schema.StringAttribute{
				Computed: true,
			},
			"entity_id": schema.StringAttribute{
				Computed: true,
			},
			"default_role_id": schema.StringAttribute{
				Computed: true,
			},
			"enable_sso_role_mapping": schema.BoolAttribute{
				Computed: true,
			},
			"signing_certificate": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *ssoConfigurationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data models.SSOConfiguration
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ssoConfig, err := d.client.GetSSOConfigurationWithoutID()
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read WarpStream SSO configuration", err.Error())
		return
	}

	data.ID = types.StringValue(ssoConfig.ID)
	data.EnableSSORoleMapping = types.BoolValue(ssoConfig.EnableSSORoleMapping)
	data.DefaultRoleID = types.StringValue(ssoConfig.DefaultRoleID)
	data.EntityID = types.StringValue(ssoConfig.EntityID)
	data.SAMLURL = types.StringValue(ssoConfig.SAMLURL)
	data.SSOIdentifier = types.StringValue(ssoConfig.SSOIdentifier)
	data.SigningCertificate = types.StringValue(ssoConfig.SigningCertificate)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *ssoConfigurationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
