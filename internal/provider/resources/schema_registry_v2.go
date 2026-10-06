package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

func NewSchemaRegistryV2Resource() resource.Resource {
	base := &schemaRegistryResource{}
	return &keyOwningResource{base: base, create: base.createV2, typeName: "_schema_registry_v2", kind: agentKeyKind}
}

// createV2 is a copy of Create that calls CreateVirtualClusterV2, which also creates a v2 agent key.
// Keep it in sync with Create.
func (r *schemaRegistryResource) createV2(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) (objectID string, key *api.APIKey) {
	var plan models.SchemaRegistryResource
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cloudPlan models.VirtualClusterSingleRegionCloud
	diags = plan.Cloud.As(ctx, &cloudPlan, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new virtual cluster
	cluster, agentKey, err := r.client.CreateVirtualClusterV2(
		plan.Name.ValueString(),
		api.ClusterParameters{
			Type:   api.VirtualClusterTypeSchemaRegistry,
			Tier:   plan.Tier.ValueString(),
			Region: cloudPlan.Region.ValueStringPointer(),
			Cloud:  cloudPlan.Provider.ValueString(),
		})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Schema Registry",
			fmt.Sprintf("Could not create WarpStream Schema Registry Virtual Cluster, unexpected error: %v", err),
		)
		return
	}
	objectID, key = cluster.ID, agentKey

	cluster, err = r.client.GetVirtualCluster(cluster.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			fmt.Sprintf("Could not get Virtual Cluster %s: %v", cluster.ID, err),
		)
		return
	}

	state := models.SchemaRegistryResource{
		ID:          types.StringValue(cluster.ID),
		Name:        types.StringValue(cluster.Name),
		Tier:        types.StringValue(cluster.Tier),
		CreatedAt:   types.StringValue(cluster.CreatedAt),
		Cloud:       plan.Cloud,
		WorkspaceID: types.StringValue(cluster.WorkspaceID),
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
	return
}
