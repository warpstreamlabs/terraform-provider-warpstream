package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/shared"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &virtualClusterResource{}
	_ resource.ResourceWithConfigure   = &virtualClusterResource{}
	_ resource.ResourceWithImportState = &virtualClusterResource{}
)

// NewVirtualClusterResource is a helper function to simplify the provider implementation.
func NewVirtualClusterResource() resource.Resource {
	return &virtualClusterResource{}
}

// virtualClusterResource is the resource implementation.
type virtualClusterResource struct {
	client *api.Client
}

// Configure adds the provider configured client to the data source.
func (r *virtualClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *virtualClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_cluster"
}

var (
	agentKeyResourceSchema = schema.NestedAttributeObject{
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
			"virtual_cluster_id": schema.StringAttribute{
				Computed: true,
			},
			"created_at": schema.StringAttribute{
				Computed: true,
			},
		},
	}
	cloudSchema = schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"provider": schema.StringAttribute{
				Description: "Cloud Provider. Valid providers are: `aws` (default), `gcp`, and `azure`.",
				Computed:    true,
				Optional:    true,
				Default:     stringdefault.StaticString("aws"),
				Validators: []validator.String{
					stringvalidator.OneOf("aws", "gcp", "azure"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				Description: "Cloud Region. Defaults to null. Can't be set if `region_group` is set.",
				Computed:    false,
				Optional:    true,
				Required:    false,
				Default:     nil,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("region_group")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region_group": schema.StringAttribute{
				Description: "Cloud Region Group. Defaults to null. Can't be set if `region` is set.",
				Computed:    false,
				Optional:    true,
				Required:    false,
				Default:     nil,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("region")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Description: "Virtual Cluster Cloud Location.",
		Optional:    true,
		Computed:    true,
		Default: objectdefault.StaticValue(
			types.ObjectValueMust(
				models.VirtualClusterCloud{}.AttributeTypes(),
				models.VirtualClusterCloud{}.DefaultObject(),
			)),
	}
)

// Schema defines the schema for the resource.
func (r *virtualClusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create, update and delete virtual clusters.

The WarpStream provider must be authenticated with an application key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Virtual Cluster ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Virtual Cluster Name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{utils.ValidClusterName()},
			},
			"type": schema.StringAttribute{
				Description: "Virtual Cluster Type. Currently, the only valid virtual cluster types is `byoc` (default).",
				Computed:    true,
				Optional:    true,
				Default:     stringdefault.StaticString(api.VirtualClusterTypeBYOC),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(api.VirtualClusterTypeBYOC),
				},
			},
			"tier": schema.StringAttribute{
				Description: "Virtual Cluster Tier. Currently, the valid virtual cluster tiers are `dev`, `pro`, `fundamentals`, and `enterprise`.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(
						api.VirtualClusterTierDev,
						api.VirtualClusterTierLegacy,
						api.VirtualClusterTierFundamentals,
						api.VirtualClusterTierPro,
						api.VirtualClusterTierEnterprise,
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
			"agent_pool_id": schema.StringAttribute{
				Description: "Agent Pool ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"agent_pool_name": schema.StringAttribute{
				Description: "Agent Pool Name.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Virtual Cluster Creation Timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"default": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.MapAttribute{
				Description: "Tags associated with the virtual cluster.",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"configuration": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"auto_create_topic": schema.BoolAttribute{
						Description: "Enable topic autocreation feature, defaults to `true`.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(true),
					},
					"default_num_partitions": schema.Int64Attribute{
						Description: "Number of partitions created by default.",
						Optional:    true,
						Computed:    true,
						Default:     int64default.StaticInt64(1),
					},
					"default_retention_millis": schema.Int64Attribute{
						Description: "Default retention for topics that are created automatically using Kafka's topic auto-creation feature.",
						Optional:    true,
						Computed:    true,
						Default:     int64default.StaticInt64(86400000),
					},
					"enable_acls": schema.BoolAttribute{
						Description: "Enable ACLs, defaults to `false`. See [Configure ACLs](https://docs.warpstream.com/warpstream/configuration/configure-acls)",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"enable_deletion_protection": schema.BoolAttribute{
						Description: "Enable deletion protection, defaults to `false`. If set to true, it is impossible to delete this cluster. enable_deletion_protection needs to be set to false before deleting the cluster.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"enable_soft_topic_deletion": schema.BoolAttribute{
						Description: "Enable soft deletion for topics. Defaults to `true`. If true, topic deletion will be a soft deletion. For clusters with the Fundamentals tier or above, it will be possible to restore topics for some time after deletion. If false, deleting a topic will immediately delete of all of its data, with no way to recover it.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(true),
					},
					"soft_delete_topic_ttl_hours": schema.Int64Attribute{
						Description: "If enable_soft_topic_deletion is true, a deleted topic's data will be kept for this many hours before being irrecoverably deleted. Defaults to 24 hours.",
						Optional:    true,
						Computed:    true,
						Default:     int64default.StaticInt64(24),
					},
				},
				Description: "Virtual Cluster Configuration.",
				Optional:    true,
				Computed:    true,
				Default: objectdefault.StaticValue(
					types.ObjectValueMust(
						models.VirtualClusterConfiguration{}.AttributeTypes(),
						models.VirtualClusterConfiguration{}.DefaultObject(),
					)),
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud": cloudSchema,
			"bootstrap_url": schema.StringAttribute{
				Description: "Bootstrap URL to connect to the Virtual Cluster.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_id": shared.VirtualClusterWorkspaceIDSchema,
		},
	}
}

// Create a new resource.
func (r *virtualClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan models.VirtualClusterResource
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
	cluster, err := r.client.CreateVirtualCluster(
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

	// Map response body to schema and populate Computed attribute values
	state := models.VirtualClusterResource{
		ID:            types.StringValue(cluster.ID),
		Name:          types.StringValue(cluster.Name),
		Type:          types.StringValue(cluster.Type),
		AgentKeys:     plan.AgentKeys,
		AgentPoolID:   types.StringValue(cluster.AgentPoolID),
		AgentPoolName: types.StringValue(cluster.AgentPoolName),
		CreatedAt:     types.StringValue(cluster.CreatedAt),
		Default:       types.BoolValue(cluster.Name == "vcn_default"),
		WorkspaceID:   types.StringValue(cluster.WorkspaceID),
		Cloud:         cloudValue,
		Tags:          plan.Tags,
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

	agentKeysState, ok := models.MapToAgentKeys(cluster.AgentKeys, &resp.Diagnostics)
	if !ok { // Diagnostics handled by helper.
		return
	}

	diags = resp.State.SetAttribute(ctx, path.Root("agent_keys"), agentKeysState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Apply configuration last, which will update the API and read back actual
	// values. This is critical when the API modifies values (e.g., setting TTL
	// to 0 when topic soft deletion is disabled).
	stateWithPlan := state
	stateWithPlan.Configuration = plan.Configuration
	r.applyConfiguration(ctx, stateWithPlan, &resp.State, &resp.Diagnostics)
}

func getCloudValue(cluster *api.VirtualCluster) (basetypes.ObjectValue, diag.Diagnostics) {
	var regionGroup *string
	var region *string
	if cluster.ClusterRegion.IsMultiRegion {
		regionGroup = &cluster.ClusterRegion.RegionGroup.Name
	} else {
		region = &cluster.ClusterRegion.Region.Name
	}

	cloudValue, diagnostics := types.ObjectValue(
		models.VirtualClusterCloud{}.AttributeTypes(),
		map[string]attr.Value{
			"provider":     types.StringValue(cluster.CloudProvider),
			"region":       types.StringPointerValue(region),
			"region_group": types.StringPointerValue(regionGroup),
		},
	)
	return cloudValue, diagnostics
}

// Read refreshes the Terraform state with the latest data.
func (r *virtualClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.VirtualClusterResource
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

	// Overwrite Virtual Cluster with refreshed state
	state.ID = types.StringValue(cluster.ID)
	state.Name = types.StringValue(cluster.Name)
	state.Type = types.StringValue(cluster.Type)
	state.AgentPoolID = types.StringValue(cluster.AgentPoolID)
	state.AgentPoolName = types.StringValue(cluster.AgentPoolName)
	state.CreatedAt = types.StringValue(cluster.CreatedAt)
	state.Default = types.BoolValue(cluster.Name == "vcn_default")
	state.WorkspaceID = types.StringValue(cluster.WorkspaceID)

	if cluster.BootstrapURL != nil {
		state.BootstrapURL = types.StringValue(*cluster.BootstrapURL)
	}

	cloudValue, diagnostics := getCloudValue(cluster)
	if diagnostics != nil {
		resp.Diagnostics.Append(diagnostics...)
		return
	}
	state.Cloud = cloudValue

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readConfiguration(ctx, *cluster, &resp.State, &resp.Diagnostics)
	r.readTags(ctx, *cluster, &resp.State, &resp.Diagnostics)

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

// Update updates the resource and sets the updated Terraform state on success.
func (r *virtualClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan models.VirtualClusterResource
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state models.VirtualClusterResource
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Rename virtual cluster if name has changed.
	if plan.Name.ValueString() != state.Name.ValueString() {
		err := r.client.RenameVirtualCluster(state.ID.ValueString(), plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Renaming WarpStream Virtual Cluster",
				"Could not rename WarpStream Virtual Cluster, unexpected error: "+err.Error(),
			)
			return
		}
		state.Name = plan.Name
		diags = resp.State.SetAttribute(ctx, path.Root("name"), state.Name)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Update virtual cluster configuration
	r.applyConfiguration(ctx, plan, &resp.State, &resp.Diagnostics)

	// Update tags if they have changed
	if !plan.Tags.IsUnknown() && !state.Tags.IsUnknown() && !plan.Tags.Equal(state.Tags) {
		stateWithPlanTags := state
		stateWithPlanTags.Tags = plan.Tags
		r.applyTags(ctx, stateWithPlanTags, &resp.State, &resp.Diagnostics)
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *virtualClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state models.VirtualClusterResource
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing virtual cluster
	err := r.client.DeleteVirtualCluster(state.ID.ValueString(), state.Name.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Virtual Cluster",
			"Could not delete WarpStream Virtual Cluster, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *virtualClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *virtualClusterResource) readConfiguration(ctx context.Context, cluster api.VirtualCluster, state *tfsdk.State, respDiags *diag.Diagnostics) {
	// Get virtual cluster configuration
	cfg, err := r.client.GetConfiguration(cluster)
	if err != nil {
		respDiags.AddError(
			"Unable to Read configuration of Virtual Cluster with ID="+cluster.ID,
			err.Error(),
		)
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Configuration: %+v", *cfg))

	cfgState := models.VirtualClusterConfiguration{
		AclsEnabled:              types.BoolValue(cfg.AclsEnabled),
		AutoCreateTopic:          types.BoolValue(cfg.AutoCreateTopic),
		DefaultNumPartitions:     types.Int64Value(cfg.DefaultNumPartitions),
		DefaultRetention:         types.Int64Value(cfg.DefaultRetentionMillis),
		EnableDeletionProtection: types.BoolValue(cfg.EnableDeletionProtection),
		SoftDeleteTopicEnable:    types.BoolValue(cfg.SoftDeleteTopicEnable),
		SoftDeleteTopicTTLHours:  types.Int64Value(cfg.SoftDeleteTopicTTLHours),
	}

	// Set configuration state
	diags := state.SetAttribute(ctx, path.Root("configuration"), cfgState)
	respDiags.Append(diags...)

	// Set tier
	diags = state.SetAttribute(ctx, path.Root("tier"), types.StringValue(cfg.Tier))
	respDiags.Append(diags...)
}

func (r *virtualClusterResource) applyConfiguration(ctx context.Context, plan models.VirtualClusterResource, state *tfsdk.State, respDiags *diag.Diagnostics) {
	cluster := plan.Cluster()

	// If configuration plan is empty, just retrieve it from API
	if plan.Configuration.IsNull() {
		tflog.Info(ctx, "No virtual cluster configuration provided")
		r.readConfiguration(ctx, cluster, state, respDiags)
		return
	}

	// Retrieve configuration values from plan
	var cfgPlan models.VirtualClusterConfiguration
	diags := plan.Configuration.As(ctx, &cfgPlan, basetypes.ObjectAsOptions{})
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}

	// Update virtual cluster configuration
	cfg := &api.VirtualClusterConfiguration{
		AclsEnabled:              cfgPlan.AclsEnabled.ValueBool(),
		AutoCreateTopic:          cfgPlan.AutoCreateTopic.ValueBool(),
		DefaultNumPartitions:     cfgPlan.DefaultNumPartitions.ValueInt64(),
		DefaultRetentionMillis:   cfgPlan.DefaultRetention.ValueInt64(),
		EnableDeletionProtection: cfgPlan.EnableDeletionProtection.ValueBool(),
		SoftDeleteTopicEnable:    cfgPlan.SoftDeleteTopicEnable.ValueBool(),
		SoftDeleteTopicTTLHours:  cfgPlan.SoftDeleteTopicTTLHours.ValueInt64(),
	}
	cfg.Tier = plan.Tier.ValueString()
	err := r.client.UpdateConfiguration(*cfg, cluster)
	if err != nil {
		respDiags.AddError(
			"Error Updating WarpStream Virtual Cluster Configuration",
			"Could not update WarpStream Virtual Cluster Configuration, unexpected error: "+err.Error(),
		)
		return
	}

	// Retrieve updated virtual cluster configuration
	r.readConfiguration(ctx, cluster, state, respDiags)
}

func (r *virtualClusterResource) readTags(ctx context.Context, cluster api.VirtualCluster, state *tfsdk.State, respDiags *diag.Diagnostics) {
	tags, err := r.client.GetTags(cluster)
	if err != nil {
		respDiags.AddError(
			"Unable to Read tags of Virtual Cluster with ID="+cluster.ID,
			err.Error(),
		)
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Tags: %+v", tags))

	tagsMap := make(map[string]attr.Value)
	for k, v := range tags {
		tagsMap[k] = types.StringValue(v)
	}

	tagsValue, diags := types.MapValue(types.StringType, tagsMap)
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}

	diags = state.SetAttribute(ctx, path.Root("tags"), tagsValue)
	respDiags.Append(diags...)
}

func (r *virtualClusterResource) applyTags(ctx context.Context, state models.VirtualClusterResource, respState *tfsdk.State, respDiags *diag.Diagnostics) {
	// Skip if tags are unknown (during import)
	if state.Tags.IsUnknown() {
		return
	}

	cluster := state.Cluster()

	var tagsMap map[string]string
	diags := state.Tags.ElementsAs(ctx, &tagsMap, false)
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}

	err := r.client.UpdateTags(tagsMap, cluster)
	if err != nil {
		respDiags.AddError(
			"Error Updating WarpStream Virtual Cluster Tags",
			"Could not update WarpStream Virtual Cluster Tags, unexpected error: "+err.Error(),
		)
		return
	}

	// Read updated tags
	r.readTags(ctx, cluster, respState, respDiags)
}
