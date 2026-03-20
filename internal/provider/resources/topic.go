package resources

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

const (
	EnableDeletionProtectionConfigKey = "warpstream.deletion.protection.enabled"
)

const (
	CleanupPolicyConfigKey = "cleanup.policy"
)

var (
	_ resource.Resource                = &topicResource{}
	_ resource.ResourceWithConfigure   = &topicResource{}
	_ resource.ResourceWithImportState = &topicResource{}
	_ resource.ResourceWithModifyPlan  = &topicResource{}
)

func NewTopicResource() resource.Resource {
	return &topicResource{}
}

type topicResource struct {
	client *api.Client
}

func (r *topicResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *topicResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_topic"
}

func (r *topicResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create, update and delete a topic.

The WarpStream provider must be authenticated with an application key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Topic ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"virtual_cluster_id": schema.StringAttribute{
				Description: "Virtual Cluster ID associated with the Topic.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.StartsWithAndAlphanumeric("vci_")},
			},
			"topic_name": schema.StringAttribute{
				Description: "Topic Name",
				Required:    true,

				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"partition_count": schema.Int64Attribute{
				Description: "Partition Count of the topic.",
				Required:    true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplaceIf(func(ctx context.Context, ir planmodifier.Int64Request, rrifr *int64planmodifier.RequiresReplaceIfFuncResponse) {
						if ir.PlanValue.ValueInt64() < ir.StateValue.ValueInt64() {
							rrifr.RequiresReplace = true
						}
					},
						"If the value of partition_count is decreased, Terraform will destroy and recreate the resource.",
						"If the value of partition_count is decreased, Terraform will destroy and recreate the resource.",
					),
				},
			},
			"enable_deletion_protection": schema.BoolAttribute{
				Description: "If enabled, WarpStream will refuse to delete this topic.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
		Blocks: map[string]schema.Block{
			// Using a set because topic configs don't have any defined order so a list can't be used
			// Golang will still treat it as a list so nothing changes code wise but terraform will
			// understand if things change order to not change anything
			"config": schema.SetNestedBlock{
				Description: "Configuration of the topic. See [WarpStream Topic Configuration](https://docs.warpstream.com/warpstream/kafka/reference/protocol-and-feature-support/topic-configuration-reference) for a list of supported configurations.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"value": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
		},
	}
}

func (r *topicResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip validation on create (no prior state) or destroy (no plan).
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var plan models.Topic
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state models.Topic
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateCleanupPolicyChange(state.Config, plan.Config); err != nil {
		resp.Diagnostics.AddError(
			"Invalid cleanup.policy Transition",
			err.Error(),
		)
		return
	}
}

func validateCleanupPolicyChange(oldConfig, newConfig []models.TopicConfig) error {
	oldPolicy := getCleanupPolicy(oldConfig)
	newPolicy := getCleanupPolicy(newConfig)

	// If either config doesn't specify cleanup.policy, skip validation.
	// The API does not always return a default cleanup.policy, so we can only
	// validate transitions when both the old and new configs explicitly set it.
	if oldPolicy == "" || newPolicy == "" {
		return nil
	}

	oldHasCompact := policyHasCompact(oldPolicy)
	newHasCompact := policyHasCompact(newPolicy)

	if oldHasCompact && !newHasCompact {
		return fmt.Errorf(
			"Cannot change cleanup.policy from %q to %q: a compacted topic cannot be made non-compacted.",
			oldPolicy, newPolicy)
	}
	if !oldHasCompact && newHasCompact {
		return fmt.Errorf(
			"Cannot change cleanup.policy from %q to %q: a non-compacted topic cannot be made compacted.",
			oldPolicy, newPolicy)
	}
	return nil
}

// getCleanupPolicy extracts the cleanup.policy value from a topic's config slice.
// Returns an empty string if cleanup.policy is not set.
func getCleanupPolicy(configs []models.TopicConfig) string {
	for _, c := range configs {
		if c.Name.ValueString() == CleanupPolicyConfigKey {
			return c.Value.ValueString()
		}
	}
	return ""
}

// policyHasCompact returns true if the cleanup.policy value contains "compact"
// as one of its comma-separated components.
func policyHasCompact(policy string) bool {
	for _, part := range strings.Split(policy, ",") {
		if strings.TrimSpace(part) == "compact" {
			return true
		}
	}
	return false
}

// filterConfigsToPlan filters API-returned configs to only include those
// the user explicitly declared in their Terraform config. This prevents
// Terraform from seeing "inconsistent result after apply" errors when the
// API returns configs that weren't in the plan.
func filterConfigsToPlan(apiConfigs map[string]*string, planConfigs []models.TopicConfig) map[string]*string {
	planned := make(map[string]struct{}, len(planConfigs))
	for _, c := range planConfigs {
		planned[c.Name.ValueString()] = struct{}{}
	}
	filtered := make(map[string]*string, len(planConfigs))
	for k, v := range apiConfigs {
		if _, ok := planned[k]; ok {
			filtered[k] = v
		}
	}
	return filtered
}

func (r *topicResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan models.Topic
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	generatedId := fmt.Sprintf("%s/%s", plan.VirtualClusterID.ValueString(), plan.TopicName.ValueString())

	var configs = make(map[string]*string, len(plan.Config)+1)

	for _, config := range plan.Config {
		configs[config.Name.ValueString()] = config.Value.ValueStringPointer()
	}
	r.addDeletionProtectionInConfigMap(plan, configs)

	err := r.client.CreateTopic(plan.VirtualClusterID.ValueString(), plan.TopicName.ValueString(), int(plan.PartitionCount.ValueInt64()), configs)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating WarpStream Topic",
			"Could not create WarpStream Topic ID "+generatedId+": "+err.Error(),
		)
		return
	}

	// Read it back so it gets set in state
	topic, err := r.client.DescribeTopic(plan.VirtualClusterID.ValueString(), plan.TopicName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Topic",
			"Could not read WarpStream Topic ID "+generatedId+": "+err.Error(),
		)
		return
	}
	deletionProtectionEnabled := r.parseTopicDeletionEnableFromConfigs(topic.Configs)

	state := models.Topic{
		ID:                        types.StringValue(generatedId),
		VirtualClusterID:          plan.VirtualClusterID,
		TopicName:                 plan.TopicName,
		DeletionProtectionEnabled: types.BoolValue(deletionProtectionEnabled),
		PartitionCount:            types.Int64Value(int64(topic.PartitionCount)),
	}

	filteredConfigs := filterConfigsToPlan(topic.Configs, plan.Config)
	for configName, configValue := range filteredConfigs {
		name := types.StringValue(configName)
		value := types.StringPointerValue(configValue)
		state.Config = append(state.Config, models.TopicConfig{
			Name:  name,
			Value: value,
		})
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *topicResource) parseTopicDeletionEnableFromConfigs(configs map[string]*string) bool {
	var (
		deletionProtectionEnabled = false
		err                       error
	)
	if configs[EnableDeletionProtectionConfigKey] != nil {
		deletionProtectionEnabled, err = strconv.ParseBool(*configs[EnableDeletionProtectionConfigKey])
		if err == nil && deletionProtectionEnabled {
			deletionProtectionEnabled = true
		}
		delete(configs, EnableDeletionProtectionConfigKey)
	}
	return deletionProtectionEnabled
}

func (r *topicResource) addDeletionProtectionInConfigMap(plan models.Topic, configs map[string]*string) {
	var enableDeletionProtectionS = "false"
	if plan.DeletionProtectionEnabled.ValueBool() {
		enableDeletionProtectionS = "true"
	}
	configs[EnableDeletionProtectionConfigKey] = &enableDeletionProtectionS
}

func (r *topicResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.Topic
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	topic, err := r.client.DescribeTopic(state.VirtualClusterID.ValueString(), state.TopicName.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading WarpStream Topic",
			"Could not read WarpStream Topic ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	previousConfig := state.Config
	state = models.Topic{
		ID:               state.ID,
		VirtualClusterID: state.VirtualClusterID,
		TopicName:        state.TopicName,
		PartitionCount:   types.Int64Value(int64(topic.PartitionCount)),
	}
	state.DeletionProtectionEnabled = types.BoolValue(r.parseTopicDeletionEnableFromConfigs(topic.Configs))

	filteredConfigs := filterConfigsToPlan(topic.Configs, previousConfig)
	for configName, configValue := range filteredConfigs {
		name := types.StringValue(configName)
		value := types.StringPointerValue(configValue)
		state.Config = append(state.Config, models.TopicConfig{
			Name:  name,
			Value: value,
		})
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *topicResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan models.Topic
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state models.Topic
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var newPartitionCount *int
	if !plan.PartitionCount.Equal(state.PartitionCount) {
		pCount := int(plan.PartitionCount.ValueInt64())
		newPartitionCount = &pCount
	}

	var configs = make(map[string]*string, len(plan.Config)+1)
	r.addDeletionProtectionInConfigMap(plan, configs)

	for _, config := range plan.Config {
		configs[config.Name.ValueString()] = config.Value.ValueStringPointer()
	}

	// Update topic resource
	err := r.client.UpdateTopic(plan.VirtualClusterID.ValueString(), plan.TopicName.ValueString(), newPartitionCount, configs)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating WarpStream Topic",
			"Could not create WarpStream Topic ID "+plan.ID.String()+": "+err.Error(),
		)
		return
	}

	// Read it back so it gets set in state
	topic, err := r.client.DescribeTopic(plan.VirtualClusterID.ValueString(), plan.TopicName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Topic",
			"Could not read WarpStream Topic ID "+plan.ID.String()+": "+err.Error(),
		)
		return
	}

	state = models.Topic{
		ID:               plan.ID,
		VirtualClusterID: plan.VirtualClusterID,
		TopicName:        plan.TopicName,
		PartitionCount:   types.Int64Value(int64(topic.PartitionCount)),
	}
	state.DeletionProtectionEnabled = types.BoolValue(r.parseTopicDeletionEnableFromConfigs(topic.Configs))
	filteredConfigs := filterConfigsToPlan(topic.Configs, plan.Config)
	for configName, configValue := range filteredConfigs {
		name := types.StringValue(configName)
		value := types.StringPointerValue(configValue)
		state.Config = append(state.Config, models.TopicConfig{
			Name:  name,
			Value: value,
		})
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *topicResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state models.Topic
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteTopic(state.VirtualClusterID.ValueString(), state.TopicName.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Topic",
			"Could not delete WarpStream Topic, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *topicResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected an ID in the format virtual_cluster_id/topic_name",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("virtual_cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("topic_name"), parts[1])...)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
