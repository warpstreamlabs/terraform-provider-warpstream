package resources

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &ssoConfigurationResource{}
	_ resource.ResourceWithConfigure   = &ssoConfigurationResource{}
	_ resource.ResourceWithImportState = &ssoConfigurationResource{}
)

// NewSSOConfigurationResource is a helper function to simplify the provider implementation.
func NewSSOConfigurationResource() resource.Resource {
	return &ssoConfigurationResource{}
}

// ssoConfigurationResource is the resource implementation.
type ssoConfigurationResource struct {
	client *api.Client
}

// Configure adds the provider configured client to the data source.
func (r *ssoConfigurationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Metadata returns the resource type name.
func (r *ssoConfigurationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_configuration"
}

// Schema defines the schema for the resource.
func (r *ssoConfigurationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create, update and delete sso configurations.

The WarpStream provider must be authenticated with an account key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "SSO configuration ID.",
				Computed:    true,
				Required:    false,
				Optional:    false,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sso_identifier": schema.StringAttribute{
				Description: "SSO Identifier. " +
					"The unique SSO identifier that will be used to identify your team.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"saml_url": schema.StringAttribute{
				Description: "SAML Sign In URL. " +
					"The SSO SAML Protocol URL for your SAML provider..",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"entity_id": schema.StringAttribute{
				Description: "SAML Entity ID. " +
					"The Entity ID for your SAML Protocol provider.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"default_role_id": schema.StringAttribute{
				Description: "Default Role ID. " +
					"The Default Role of a user when they login to SSO for the first time. You can use https://docs.warpstream.com/warpstream/reference/api-reference/user-roles/list to get the Role IDs.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enable_sso_role_mapping": schema.BoolAttribute{
				Description: "Enable SSO Role Mapping." +
					"When enabled, user roles will be automatically assigned or unassigned based on SSO group membership.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"signing_certificate": schema.StringAttribute{
				Description: "X509 Signing Certificate. " +
					"SAML Protocol server public key encoded in PEM format.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *ssoConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan models.SSOConfiguration
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new configuration
	ssoConfigurationID, err := r.client.CreateSSOConfiguration(api.SSOConfigurationCreateRequest{
		SSOIdentifier:        plan.SSOIdentifier.ValueString(),
		EntityID:             plan.EntityID.ValueString(),
		SAMLURL:              plan.SAMLURL.ValueString(),
		DefaultRoleID:        plan.DefaultRoleID.ValueString(),
		EnableSSORoleMapping: plan.EnableSSORoleMapping.ValueBool(),
		SigningCertificate:   plan.SigningCertificate.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream SSO Configuration",
			"Could not create WarpStream SSO Configuration, unexpected error: "+err.Error(),
		)
		return
	}

	// Describe created config
	ssoConfig, err := r.client.GetSSOConfiguration(ssoConfigurationID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream SSO Configuration",
			"Could not read WarpStream SSO Configuration ID "+ssoConfigurationID+": "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	state := models.SSOConfiguration{
		ID:                   types.StringValue(ssoConfig.ID),
		SSOIdentifier:        types.StringValue(ssoConfig.SSOIdentifier),
		EntityID:             types.StringValue(ssoConfig.EntityID),
		SAMLURL:              types.StringValue(ssoConfig.SAMLURL),
		DefaultRoleID:        types.StringValue(ssoConfig.DefaultRoleID),
		EnableSSORoleMapping: types.BoolValue(ssoConfig.EnableSSORoleMapping),
		SigningCertificate:   types.StringValue(ssoConfig.SigningCertificate),
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *ssoConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.SSOConfiguration
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ssoConfig, err := r.client.GetSSOConfiguration(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading WarpStream SSO Configuration",
			"Could not read WarpStream SSO Configuration ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite model with refreshed state
	state = models.SSOConfiguration{
		ID:                   types.StringValue(ssoConfig.ID),
		SSOIdentifier:        types.StringValue(ssoConfig.SSOIdentifier),
		EntityID:             types.StringValue(ssoConfig.EntityID),
		SAMLURL:              types.StringValue(ssoConfig.SAMLURL),
		DefaultRoleID:        types.StringValue(ssoConfig.DefaultRoleID),
		EnableSSORoleMapping: types.BoolValue(ssoConfig.EnableSSORoleMapping),
		SigningCertificate:   types.StringValue(ssoConfig.SigningCertificate),
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update config.
func (r *ssoConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan.
	var plan models.SSOConfiguration
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update config.
	err := r.client.UpdateSSOConfiguration(api.SSOConfigurationUpdateRequest{
		SSOConnectionID:      plan.ID.ValueString(),
		EntityID:             plan.EntityID.ValueString(),
		SAMLURL:              plan.SAMLURL.ValueString(),
		DefaultRoleID:        plan.DefaultRoleID.ValueString(),
		EnableSSORoleMapping: plan.EnableSSORoleMapping.ValueBool(),
		SigningCertificate:   plan.SigningCertificate.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating WarpStream SSO Configuration",
			"Could not update WarpStream SSO Configuration, unexpected error: "+err.Error(),
		)
		return
	}

	// Set state to fully populated data.
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *ssoConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state models.SSOConfiguration
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing workspace
	err := r.client.DeleteSSOConfiguration(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream SSO Configuration",
			"Could not delete WarpStream SSO Configuration, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *ssoConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
