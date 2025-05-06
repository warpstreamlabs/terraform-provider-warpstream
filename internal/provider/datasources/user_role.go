package datasources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/resources"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &userRoleDataSource{}
	_ datasource.DataSourceWithConfigure = &userRoleDataSource{}
)

// helper function to simplify the provider implementation.
func NewUserRoleDataSource() datasource.DataSource {
	return &userRoleDataSource{}
}

// userRoleDataSource is the data source implementation.
type userRoleDataSource struct {
	client *api.Client
}

// Metadata returns the data source type name.
func (d *userRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_role"
}

var grantSchema = schema.NestedAttributeObject{
	Attributes: map[string]schema.Attribute{
		"workspace_id": schema.StringAttribute{
			Description: "ID of a workspace that the role has access to.",
			Computed:    true,
		},
		"grant_type": schema.StringAttribute{
			Description: "Level of access inside the workspace. Current options are: " + strings.Join(resources.ManagedGrantNames, " and "),
			Computed:    true,
		},
	},
}

// Schema defines the schema for the data source.
func (d *userRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This data source lists User Roles and their respective grants.

The WarpStream provider must be authenticated with an account key to read this data source.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "User Role ID. Exactly one of id or name must be provided.",
				Computed:    true,
				Optional:    true,
			},
			"name": schema.StringAttribute{
				Description: "User Role Name. Exactly one of id or name must be provided." +
					"Unique across WarpStream account. " +
					"Contain spaces, hyphens, underscores or alphanumeric characters only. " +
					"Between 3 and 60 characters in length.",
				Computed:   true,
				Optional:   true,
				Validators: []validator.String{utils.ValidUserRoleName()},
			},
			"access_grants": schema.ListNestedAttribute{
				Description:  "List of grants defining the role's access level inside each workspace.",
				Computed:     true,
				Optional:     true,
				NestedObject: grantSchema,
			},
			"created_at": schema.StringAttribute{
				Description: "User Role Creation Timestamp.",
				Computed:    true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *userRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data models.UserRole
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleID := data.ID.ValueString()
	roleName := data.Name.ValueString()

	if roleID == "" && roleName == "" {
		resp.Diagnostics.AddError("Unable to Read WarpStream UserRole", "Either id or name must be provided.")
		return
	}

	if roleID != "" && roleName != "" {
		resp.Diagnostics.AddError("Unable to Read WarpStream UserRole", "Only one of id or name must be provided.")
		return
	}

	var (
		role *api.UserRole
		err  error
	)

	if roleID != "" {
		role, err = d.client.GetUserRole(roleID)
	} else {
		role, err = d.client.FindUserRole(roleName)
	}

	if err != nil {
		resp.Diagnostics.AddError("Unable to Read WarpStream UserRole", err.Error())
		return
	}

	state := models.UserRole{
		ID:        types.StringValue(role.ID),
		Name:      types.StringValue(role.Name),
		CreatedAt: types.StringValue(role.CreatedAt),
	}

	grantModels := make([]models.UserRoleGrant, 0, len(role.AccessGrants))
	for _, grant := range role.AccessGrants {
		grantModels = append(grantModels, models.UserRoleGrant{
			WorkspaceID: types.StringValue(grant.WorkspaceID),
			GrantType:   types.StringValue(grant.ManagedGrantKey),
		})
	}
	state.AccessGrants = grantModels

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *userRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
