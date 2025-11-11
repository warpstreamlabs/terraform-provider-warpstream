package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/shared"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &tableFlowResource{}
	_ resource.ResourceWithConfigure   = &tableFlowResource{}
	_ resource.ResourceWithImportState = &tableFlowResource{}
)

type tableFlowResource struct {
	client *api.Client
}

func NewTableFlowResource() resource.Resource {
	return &tableFlowResource{}
}

func (r *tableFlowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tableflow_cluster"
}

func (r *tableFlowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tableFlowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create, update and delete TableFlow clusters.

The WarpStream provider must be authenticated with an application key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "TableFlow ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "TableFlow Cluster Name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.ValidTableFlowName()},
			},
			"tier": schema.StringAttribute{
				Description: "Virtual Cluster Tier. Currently, the valid virtual cluster tiers are `dev`, `pro`, and `fundamentals`.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(
						api.VirtualClusterTierDev,
						api.VirtualClusterTierFundamentals,
						api.VirtualClusterTierPro,
					),
				},
			},
			"agent_keys": schema.ListNestedAttribute{
				Description:  "List of keys to authenticate an agent with this cluster.",
				Computed:     true,
				NestedObject: agentKeyResourceSchema,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Virtual Cluster Creation Timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud":        cloudSchema,
			"workspace_id": shared.VirtualClusterWorkspaceIDSchema,
		},
	}
}

func (r *tableFlowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan models.TableFlowResource
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cloudPlan models.VirtualClusterCloud
	diags = plan.Cloud.As(ctx, &cloudPlan, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new virtual cluster
	cluster, err := r.client.CreateVirtualCluster(
		plan.Name.ValueString(),
		api.ClusterParameters{
			Type:   api.VirtualClusterTypeTableFlow,
			Tier:   plan.Tier.ValueString(),
			Region: cloudPlan.Region.ValueStringPointer(),
			Cloud:  cloudPlan.Provider.ValueString(),
		})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream TableFlow",
			fmt.Sprintf("Could not create WarpStream TableFlow Virtual Cluster, unexpected error: %v", err),
		)
		return
	}

	cluster, err = r.client.GetVirtualCluster(cluster.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			fmt.Sprintf("Could not get Virtual Cluster %s: %v", cluster.ID, err),
		)
		return
	}

	state := models.TableFlowResource{
		ID:          types.StringValue(cluster.ID),
		Name:        types.StringValue(cluster.Name),
		Tier:        types.StringValue(cluster.Tier),
		AgentKeys:   plan.AgentKeys,
		CreatedAt:   types.StringValue(cluster.CreatedAt),
		Cloud:       plan.Cloud,
		WorkspaceID: types.StringValue(cluster.WorkspaceID),
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentKeysState, ok := models.MapToAgentKeys(cluster.AgentKeys, &resp.Diagnostics)
	if !ok { // Diagnostics handled by helper.
		return
	}

	diags = resp.State.SetAttribute(ctx, path.Root("agent_keys"), agentKeysState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *tableFlowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.TableFlowResource
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cluster *api.VirtualCluster

	// Crossplane.io creates terraform state manually with empty IDs. There is
	// no terraform standard to handle empty IDs and our API does not handle
	// them in a way that is useful. Other TF providers are a mixed bag when
	// handling empty IDs, so let's explicitly handle them.
	if state.ID.ValueString() == "" {
		var err error
		cluster, err = r.client.FindVirtualCluster(state.Name.ValueString())
		if err != nil {
			if errors.Is(err, api.ErrNotFound) {
				resp.State.RemoveResource(ctx)
				return
			}

			resp.Diagnostics.AddError(
				"Error Reading WarpStream Virtual Cluster",
				"Could not read WarpStream Virtual Cluster Name "+state.Name.ValueString()+": "+err.Error(),
			)
		}
		state.ID = types.StringValue(cluster.ID)
	}

	cluster, err := r.client.GetVirtualCluster(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			"Could not read WarpStream Virtual Cluster ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.ID = types.StringValue(cluster.ID)
	state.Name = types.StringValue(cluster.Name)
	state.Tier = types.StringValue(cluster.Tier)
	state.WorkspaceID = types.StringValue(cluster.WorkspaceID)
	state.CreatedAt = types.StringValue(cluster.CreatedAt)

	cloudValue, diagnostics := types.ObjectValue(
		models.VirtualClusterCloud{}.AttributeTypes(),
		map[string]attr.Value{
			"provider": types.StringValue(cluster.CloudProvider),
			// tableflow is always single region
			"region": types.StringValue(cluster.ClusterRegion.Region.Name),
		},
	)
	if diagnostics != nil {
		resp.Diagnostics.Append(diagnostics...)
		return
	}
	state.Cloud = cloudValue

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentKeysState, ok := models.MapToAgentKeys(cluster.AgentKeys, &resp.Diagnostics)
	if !ok { // Diagnostics handled by helper.
		return
	}
	diags = resp.State.SetAttribute(ctx, path.Root("agent_keys"), agentKeysState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *tableFlowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan models.TableFlowResource
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state models.TableFlowResource
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update tier if changed
	if !plan.Tier.Equal(state.Tier) {
		err := r.client.UpdateVirtualClusterTier(state.ID.ValueString(), plan.Tier.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating WarpStream TableFlow Tier",
				"Could not update WarpStream TableFlow Tier, unexpected error: "+err.Error(),
			)
			return
		}

		// Read back the updated cluster to get the new tier
		cluster, err := r.client.GetVirtualCluster(state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading WarpStream Virtual Cluster after tier update",
				"Could not read WarpStream Virtual Cluster ID "+state.ID.ValueString()+": "+err.Error(),
			)
			return
		}

		diags = resp.State.SetAttribute(ctx, path.Root("tier"), types.StringValue(cluster.Tier))
		resp.Diagnostics.Append(diags...)
	}
}

func (r *tableFlowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state models.TableFlowResource
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVirtualCluster(state.ID.ValueString(), state.Name.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream TableFlow",
			fmt.Sprintf("Could not delete WarpStream TableFlow %s: %v", state.Name, err),
		)
		return
	}
}

func (r *tableFlowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
