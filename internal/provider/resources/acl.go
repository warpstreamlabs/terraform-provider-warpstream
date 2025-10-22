package resources

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

var (
	validACLResourceTypes   = []string{"TOPIC", "GROUP", "CLUSTER", "TRANSACTIONAL_ID", "DELEGATION_TOKEN"}
	validACLPatternTypes    = []string{"LITERAL", "PREFIXED"}
	validACLOperations      = []string{"ALL", "READ", "WRITE", "CREATE", "DELETE", "ALTER", "DESCRIBE", "CLUSTER_ACTION", "DESCRIBE_CONFIGS", "ALTER_CONFIGS", "IDEMPOTENT_WRITE"}
	validACLPermissionTypes = []string{"DENY", "ALLOW"}
)

var (
	_ resource.Resource              = &aclResource{}
	_ resource.ResourceWithConfigure = &aclResource{}
)

func NewACLResource() resource.Resource {
	return &aclResource{}
}

type aclResource struct {
	client *api.Client
}

// Configure adds the provider configured client to the data source.
func (a *aclResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	a.client = client
}

// Metadata implements resource.Resource.
func (a *aclResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acl"
}

func (a *aclResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create, read and delete ACLs related to a virtual cluster.

The WarpStream provider must be authenticated with an application key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "ACL ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"virtual_cluster_id": schema.StringAttribute{
				Description: "The ID of the Virtual Cluster that the ACL applies to.",
				Required:    true,
				Validators:  []validator.String{utils.ValidClusterID()},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host": schema.StringAttribute{
				Description:   "Host from which the principal will have access. Use * to allow access from any host.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"principal": schema.StringAttribute{
				Description:   "The principal for the ACL.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},

			"operation": schema.StringAttribute{
				Description: "The operation type for the ACL. Accepted values are: `ALL`, `READ`, `WRITE`, `CREATE`, `DELETE`, `ALTER`, `DESCRIBE`, `CLUSTER_ACTION`, `DESCRIBE_CONFIGS`, `ALTER_CONFIGS` or `IDEMPOTENT_WRITE`.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(validACLOperations...),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"permission_type": schema.StringAttribute{
				Description:   "The permission for the ACL. Accepted values are: `ALLOW` or `DENY`.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.OneOf(validACLPermissionTypes...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"resource_type": schema.StringAttribute{
				Description: "The type of the resource. Accepted values are: `TOPIC`, `GROUP`, `CLUSTER`, `TRANSACTIONAL_ID` or `DELEGATION_TOKEN`.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(validACLResourceTypes...),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"resource_name": schema.StringAttribute{
				Description:   "The resource name for the ACL",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"pattern_type": schema.StringAttribute{
				Description:   "The pattern type for the ACL. Accepted values are `LITERAL` or `PREFIXED`.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.OneOf(validACLPatternTypes...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

// Creates a new ACL resource.
func (a *aclResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan models.ACL
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new ACL
	acl, err := a.client.CreateACL(plan.VirtualClusterID.ValueString(), api.ACLRequest{
		ResourceType:   plan.ResourceType.ValueString(),
		ResourceName:   plan.ResourceName.ValueString(),
		PatternType:    plan.PatternType.ValueString(),
		Principal:      plan.Principal.ValueString(),
		Host:           plan.Host.ValueString(),
		Operation:      plan.Operation.ValueString(),
		PermissionType: plan.PermissionType.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating ACL", fmt.Sprintf("Failed to create ACL: %s", err.Error()))
		return
	}

	aclToDescribe := api.ACLRequest{
		ResourceType:   acl.ResourceType,
		ResourceName:   acl.ResourceName,
		PatternType:    acl.PatternType,
		Principal:      acl.Principal,
		Host:           acl.Host,
		Operation:      acl.Operation,
		PermissionType: acl.PermissionType,
	}

	log.Printf("ACL created with ID: %s, vc: %s", acl.ID(), plan.VirtualClusterID.ValueString())

	// Describe the created ACL
	acl, err = a.client.GetACL(plan.VirtualClusterID.String(), aclToDescribe)
	if err != nil {
		resp.Diagnostics.AddError("Error reading ACL", fmt.Sprintf("Failed to read ACL: %s", err.Error()))
		return
	}

	// Map response body to schema and populate computed attributes
	state := models.ACL{
		ID:               types.StringValue(acl.ID()),
		VirtualClusterID: plan.VirtualClusterID,
		Host:             types.StringValue(acl.Host),
		Principal:        types.StringValue(acl.Principal),
		Operation:        types.StringValue(acl.Operation),
		PermissionType:   types.StringValue(acl.PermissionType),
		ResourceType:     types.StringValue(acl.ResourceType),
		ResourceName:     types.StringValue(acl.ResourceName),
		PatternType:      types.StringValue(acl.PatternType),
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (a *aclResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.ACL
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	aclToDescribe := api.ACLRequest{
		ResourceType:   state.ResourceType.ValueString(),
		ResourceName:   state.ResourceName.ValueString(),
		PatternType:    state.PatternType.ValueString(),
		Principal:      state.Principal.ValueString(),
		Host:           state.Host.ValueString(),
		Operation:      state.Operation.ValueString(),
		PermissionType: state.PermissionType.ValueString(),
	}

	// Get the latest ACL data
	acl, err := a.client.GetACL(state.VirtualClusterID.ValueString(), aclToDescribe)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading ACL", fmt.Sprintf("Failed to read ACL: %s", err.Error()))
		return
	}

	// Overwrite ACL with refreshed state
	state = models.ACL{
		ID:               types.StringValue(acl.ID()),
		VirtualClusterID: types.StringValue(state.VirtualClusterID.String()),
		Host:             types.StringValue(acl.Host),
		Principal:        types.StringValue(acl.Principal),
		Operation:        types.StringValue(acl.Operation),
		PermissionType:   types.StringValue(acl.PermissionType),
		ResourceType:     types.StringValue(acl.ResourceType),
		ResourceName:     types.StringValue(acl.ResourceName),
		PatternType:      types.StringValue(acl.PatternType),
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (a *aclResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// ACLs are immutable. All user-set attributes are marked RequiresReplace().
	// If Update is invoked, detect drift and raise an error.
	tflog.Warn(ctx, "Update called on immutable ACL resource. This should not happen if RequiresReplace is properly configured.")

	var plan, state models.ACL
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Compare immutable fields; if any differ, signal an unexpected Update.
	if !plan.VirtualClusterID.Equal(state.VirtualClusterID) ||
		!plan.Host.Equal(state.Host) ||
		!plan.Principal.Equal(state.Principal) ||
		!plan.Operation.Equal(state.Operation) ||
		!plan.PermissionType.Equal(state.PermissionType) ||
		!plan.ResourceType.Equal(state.ResourceType) ||
		!plan.ResourceName.Equal(state.ResourceName) ||
		!plan.PatternType.Equal(state.PatternType) {
		resp.Diagnostics.AddError(
			"Immutable ACL Change Detected",
			"WarpStream ACLs are immutable; changes require resource replacement. Terraform should have planned a replace.",
		)
	}

	// Preserve prior state (no changes in-place).
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the ACL resource and removes the Terraform state on success.
func (a *aclResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state models.ACL
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the ACL
	err := a.client.DeleteACL(state.VirtualClusterID.ValueString(), api.ACLRequest{
		ResourceType:   state.ResourceType.ValueString(),
		ResourceName:   state.ResourceName.ValueString(),
		PatternType:    state.PatternType.ValueString(),
		Principal:      state.Principal.ValueString(),
		Host:           state.Host.ValueString(),
		Operation:      state.Operation.ValueString(),
		PermissionType: state.PermissionType.ValueString(),
	})
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError("Error deleting ACL", fmt.Sprintf("Failed to delete ACL: %s", err.Error()))
		return
	}
}
