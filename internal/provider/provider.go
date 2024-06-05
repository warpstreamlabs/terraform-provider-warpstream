package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &warpstreamProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &warpstreamProvider{
			version: version,
		}
	}
}

// warpstreamProvider is the provider implementation.
type warpstreamProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// warpstreamProviderModel describes the provider data model.
type warpstreamProviderModel struct {
	Token   types.String `tfsdk:"token"`
	BaseUrl types.String `tfsdk:"base_url"`
}

// Metadata returns the provider type name.
func (p *warpstreamProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "warpstream"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *warpstreamProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Description: "Base URL for WarpStream API. May also be provided via WARPSTREAM_API_URL environment variable.",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "Token for WarpStream API. May also be provided via WARPSTREAM_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

// Configure prepares a WarpStream API client for data sources and resources.
func (p *warpstreamProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config warpstreamProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown Warpstream API Token",
			"The provider cannot create the Warpstream API client as there is an unknown configuration value for the Warpstream API token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the WARPSTREAM_API_KEY environment variable.",
		)
	}

	if config.BaseUrl.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Unknown Warpstream Base URL for the API endpoint",
			"The provider cannot create the WarpStream API client as there is an unknown configuration value for the Warpstream Base URL. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the WARPSTREAM_API_URL environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	token := os.Getenv("WARPSTREAM_API_KEY")
	host := os.Getenv("WARPSTREAM_API_URL")

	if !config.BaseUrl.IsNull() {
		host = config.BaseUrl.ValueString()
	}

	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing Warpstream API Token",
			"The provider cannot create the WarpStream API client as there is a missing or empty value for the Warpstream API token. "+
				"Set the token value in the configuration or use the WARPSTREAM_API_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create a new WarpStream client using the configuration values
	client, err := api.NewClient(host, &token)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Warpstream API Client",
			"An unexpected error occurred when creating the Warpstream API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Warpstream Client Error: "+err.Error(),
		)
		return
	}

	// Make the Warpstream client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *warpstreamProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewVirtualClusterDataSource,
		NewVirtualClustersDataSource,
		NewAccountDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *warpstreamProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewVirtualClusterResource,
		NewVirtualClusterCredentialsResource,
		NewPipelineResource,
	}
}
