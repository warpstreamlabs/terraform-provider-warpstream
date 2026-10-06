package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

func NewVirtualClusterV2Resource() resource.Resource {
	base := &virtualClusterResource{}
	return &keyOwningResource{base: base, create: base.createV2, typeName: "_virtual_cluster_v2", kind: agentKeyKind}
}

// createV2 is a copy of Create that calls CreateVirtualClusterV2, which also creates a v2 agent key.
// Keep it in sync with Create.
func (r *virtualClusterResource) createV2(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) (objectID string, key *api.APIKey) {
	// Retrieve values from plan
	var plan models.VirtualClusterResource
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var configEvents types.Object
	diags = req.Config.GetAttribute(ctx, path.Root("events"), &configEvents)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	eventsManaged := eventsConfigured(configEvents, plan.Events, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var cloudPlan models.VirtualClusterCloud
	diags = plan.Cloud.As(ctx, &cloudPlan, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var tagsMap map[string]string
	if plan.Tags.IsNull() || plan.Tags.IsUnknown() {
		tagsMap = make(map[string]string)
	} else {
		diags = plan.Tags.ElementsAs(ctx, &tagsMap, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Create new virtual cluster
	cluster, agentKey, err := r.client.CreateVirtualClusterV2(
		plan.Name.ValueString(),
		api.ClusterParameters{
			Type:        plan.Type.ValueString(),
			Tier:        plan.Tier.ValueString(),
			RegionGroup: cloudPlan.RegionGroup.ValueStringPointer(),
			Region:      cloudPlan.Region.ValueStringPointer(),
			Cloud:       cloudPlan.Provider.ValueString(),
			Tags:        tagsMap,
		},
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Virtual Cluster",
			"Could not create WarpStream Virtual Cluster, unexpected error: "+err.Error(),
		)
		return
	}
	objectID, key = cluster.ID, agentKey

	// Describe created virtual cluster
	clusterID := cluster.ID
	cluster, err = r.client.GetVirtualCluster(clusterID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			"Could not read WarpStream Virtual Cluster ID "+clusterID+": "+err.Error(),
		)
		return
	}

	cloudValue, diagnostics := getCloudValue(cluster)
	if diagnostics != nil {
		resp.Diagnostics.Append(diagnostics...)
		return
	}

	// Unmanaged Events: do not seed state from the plan (which may be unknown).
	// Managed Events: seed from the resolved plan so applyEvents can write it.
	eventsForState := types.ObjectNull(models.VirtualClusterEvents{}.AttributeTypes())
	if eventsManaged {
		eventsForState = plan.Events
	}

	// Map response body to schema and populate Computed attribute values
	state := models.VirtualClusterResource{
		ID:                  types.StringValue(cluster.ID),
		Name:                types.StringValue(cluster.Name),
		Type:                types.StringValue(cluster.Type),
		AgentPoolID:         types.StringValue(cluster.AgentPoolID),
		AgentPoolName:       types.StringValue(cluster.AgentPoolName),
		CreatedAt:           types.StringValue(cluster.CreatedAt),
		Default:             types.BoolValue(cluster.Name == "vcn_default"),
		WorkspaceID:         types.StringValue(cluster.WorkspaceID),
		Configuration:       plan.Configuration,
		BrokerConfiguration: plan.BrokerConfiguration,
		Events:              eventsForState,
		Cloud:               cloudValue,
		Tags:                plan.Tags,
	}

	if cluster.BootstrapURL != nil {
		state.BootstrapURL = types.StringValue(*cluster.BootstrapURL)
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readTags(ctx, *cluster, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyConfiguration(ctx, state, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		r.readEvents(ctx, *cluster, &resp.State, &resp.Diagnostics,
			types.MapNull(types.ObjectType{AttrTypes: models.EventTypeConfig{}.AttributeTypes()}))
		return
	}

	r.applyEvents(ctx, state, &resp.State, &resp.Diagnostics, eventsManaged)
	return
}
