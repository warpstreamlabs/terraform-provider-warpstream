package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
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
	_ resource.ResourceWithModifyPlan  = &virtualClusterResource{}
)

// typedConfigToBrokerKeys maps a typed `configuration` attribute (by its tfsdk name) to
// the generic broker-config key(s) that control the SAME underlying cluster setting. It is
// used by ModifyPlan to reject setting the same setting via both a typed attribute and the
// generic `config` block. Only typed attributes that have a generic-key equivalent are
// listed; the ACL and deletion-protection attributes have no broker-config form and can
// never collide.
var typedConfigToBrokerKeys = map[string][]string{
	"auto_create_topic":              {"auto.create.topics.enable"},
	"default_num_partitions":         {"num.partitions"},
	"default_retention_millis":       {"log.retention.ms", "log.retention.minutes", "log.retention.hours"},
	"default_topic_type":             {"warpstream.default.topic.type"},
	"enable_soft_topic_deletion":     {"warpstream.soft.delete.topic.enable"},
	"soft_topic_deletion_ttl_millis": {"warpstream.soft.delete.topic.ttl.hours"},
}

// configCollision is a typed `configuration` attribute that conflicts with a generic
// `config` block key because both control the same underlying cluster setting.
type configCollision struct {
	TypedAttr  string
	GenericKey string
}

// findConfigCollisions returns every conflict between an explicitly-set typed configuration
// attribute and a generic config block key. It is pure so it can be unit-tested without the
// plugin-framework plan machinery.
func findConfigCollisions(explicitTypedAttrs map[string]struct{}, genericKeys map[string]struct{}) []configCollision {
	var out []configCollision
	for typedName, keys := range typedConfigToBrokerKeys {
		if _, set := explicitTypedAttrs[typedName]; !set {
			continue
		}
		for _, k := range keys {
			if _, dup := genericKeys[k]; dup {
				out = append(out, configCollision{TypedAttr: typedName, GenericKey: k})
			}
		}
	}
	return out
}

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

// retentionAliasKeys are the broker-config retention aliases we reject in
// `broker_configuration`: the WarpStream API always returns retention as
// `log.retention.ms`, so accepting the minutes/hours aliases would cause perpetual drift.
var retentionAliasKeys = []string{"log.retention.minutes", "log.retention.hours"}

// ModifyPlan validates and reconciles the generic `broker_configuration` map against the
// typed `configuration` attribute. It (1) rejects unsupported retention aliases, (2) rejects
// configuring the same underlying setting via both a typed attribute and the map (which the
// API rejects too), and (3) marks a typed attribute known-after-apply when its generic key
// is being added or changed, so the API-provided value can be read back without a spurious
// diff or an "inconsistent result after apply" error.
func (r *virtualClusterResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Nothing to validate on destroy.
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan models.VirtualClusterResource
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	planBroker := brokerConfigMap(ctx, plan.BrokerConfiguration, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || len(planBroker) == 0 {
		return
	}

	// Reject retention aliases; only `log.retention.ms` is accepted.
	for _, alias := range retentionAliasKeys {
		if _, ok := planBroker[alias]; ok {
			resp.Diagnostics.AddError(
				"Invalid broker configuration",
				fmt.Sprintf("`broker_configuration` key %q is not supported; specify retention as `log.retention.ms`.", alias),
			)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	genericKeys := make(map[string]struct{}, len(planBroker))
	for k := range planBroker {
		genericKeys[k] = struct{}{}
	}

	// Read the typed configuration from the raw *config* (not the plan) so that Computed
	// defaults are not mistaken for values the user explicitly set.
	var cfgObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("configuration"), &cfgObj)...)
	if resp.Diagnostics.HasError() {
		return
	}
	explicitTypedAttrs := make(map[string]struct{})
	if !cfgObj.IsNull() && !cfgObj.IsUnknown() {
		for name, v := range cfgObj.Attributes() {
			if v != nil && !v.IsNull() && !v.IsUnknown() {
				explicitTypedAttrs[name] = struct{}{}
			}
		}
	}

	collisions := findConfigCollisions(explicitTypedAttrs, genericKeys)
	for _, c := range collisions {
		resp.Diagnostics.AddError(
			"Conflicting virtual cluster configuration",
			fmt.Sprintf(
				"The setting controlled by the typed `configuration.%s` attribute is also set "+
					"via the generic `broker_configuration` key %q. Set it only one way.",
				c.TypedAttr, c.GenericKey,
			),
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Prior state's map (null on create).
	var stateBroker map[string]string
	if !req.State.Raw.IsNull() {
		var state models.VirtualClusterResource
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
		stateBroker = brokerConfigMap(ctx, state.BrokerConfiguration, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	invalidate := typedAttrsToInvalidate(planBroker, stateBroker)
	if len(invalidate) == 0 {
		return
	}

	// Mark the affected typed attributes known-after-apply in the plan.
	var planCfg types.Object
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("configuration"), &planCfg)...)
	if resp.Diagnostics.HasError() || planCfg.IsNull() || planCfg.IsUnknown() {
		return
	}
	attrs := make(map[string]attr.Value, len(planCfg.Attributes()))
	for name, v := range planCfg.Attributes() {
		attrs[name] = v
	}
	for name := range invalidate {
		switch attrs[name].(type) {
		case types.Bool:
			attrs[name] = types.BoolUnknown()
		case types.Int64:
			attrs[name] = types.Int64Unknown()
		case types.String:
			attrs[name] = types.StringUnknown()
		}
	}
	newObj, diags := types.ObjectValue(planCfg.AttributeTypes(ctx), attrs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("configuration"), newObj)...)
}

// brokerConfigMap extracts a types.Map of strings into a Go map, returning nil when the
// attribute is null or unknown.
func brokerConfigMap(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := make(map[string]string, len(m.Elements()))
	diags.Append(m.ElementsAs(ctx, &out, false)...)
	return out
}

// typedAttrsToInvalidate returns the set of typed `configuration` attribute names whose
// generic broker key is being added or changed in the plan's broker_configuration map
// (compared to prior state). Those attributes must be marked known-after-apply so the
// API-provided value can be read back without a spurious diff.
func typedAttrsToInvalidate(planBroker, stateBroker map[string]string) map[string]struct{} {
	out := map[string]struct{}{}
	for typedAttr, keys := range typedConfigToBrokerKeys {
		for _, k := range keys {
			planVal, inPlan := planBroker[k]
			if !inPlan {
				continue
			}
			if stateVal, inState := stateBroker[k]; !inState || stateVal != planVal {
				out[typedAttr] = struct{}{}
			}
		}
	}
	return out
}

var (
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
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
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
					"default_topic_type": schema.StringAttribute{
						Description: "Default topic type for new topics. Valid values are `classic` or `lightning`. If not specified, the WarpStream API defaults to `classic`. See [Lightning Topics](https://docs.warpstream.com/warpstream/kafka/advanced-agent-deployment-options/low-latency-clusters/lightning-topics)",
						Optional:    true,
						Computed:    true,
						Validators: []validator.String{
							stringvalidator.OneOf("classic", "lightning"),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"enable_acls": schema.BoolAttribute{
						Description: "Enable ACLs, defaults to `false`. See [Configure ACLs](https://docs.warpstream.com/warpstream/configuration/configure-acls)",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"enable_acl_shadowing": schema.BoolAttribute{
						Description: "Enable ACL shadowing, defaults to `false`. See [ACL Shadowing](https://docs.warpstream.com/warpstream/kafka/manage-security/configure-acls#acl-shadowing)",
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
					"soft_topic_deletion_ttl_millis": schema.Int64Attribute{
						Description: "If enable_soft_topic_deletion is true, a deleted topic's data will be kept for this many milliseconds before being irrecoverably deleted. Defaults to 24 hours.",
						Optional:    true,
						Computed:    true,
						Default:     int64default.StaticInt64(86400000),
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
				Validators: []validator.Object{
					utils.ACLModeMutualExclusion(),
				},
			},
			"events": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Description: "Enable events for this virtual cluster. Defaults to `false`.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
						PlanModifiers: []planmodifier.Bool{
							boolplanmodifier.UseStateForUnknown(),
						},
					},
					"event_types": schema.MapNestedAttribute{
						Description: "Per event type configuration. Map keys are event type names. Refer to the Events tab of the WarpStream web console for the list of valid event types.",
						Optional:    true,
						Computed:    true,
						PlanModifiers: []planmodifier.Map{
							mapplanmodifier.UseStateForUnknown(),
						},
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"enabled": schema.BoolAttribute{
									Description: "Whether this event type is enabled.",
									Optional:    true,
									Computed:    true,
									PlanModifiers: []planmodifier.Bool{
										boolplanmodifier.UseStateForUnknown(),
									},
								},
								"retention_period_nanos": schema.Int64Attribute{
									Description: "Retention period in nanoseconds for this event type.",
									Optional:    true,
									Computed:    true,
									PlanModifiers: []planmodifier.Int64{
										int64planmodifier.UseStateForUnknown(),
									},
								},
							},
						},
					},
				},
				Description: "Virtual Cluster Events Configuration.",
				Optional:    true,
				Computed:    true,
				Default: objectdefault.StaticValue(
					types.ObjectValueMust(
						models.VirtualClusterEvents{}.AttributeTypes(),
						models.VirtualClusterEvents{}.DefaultObject(),
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
			"broker_configuration": schema.MapAttribute{
				Description: "Generic cluster/broker configuration as a map of Kafka-style " +
					"config names to values (e.g. `message.max.bytes = \"1048576\"`, " +
					"`delete.topic.enable = \"true\"`). Use this for settings that don't have a " +
					"dedicated typed attribute under `configuration`, or to manage them generically. " +
					"A given setting must be set via either its typed `configuration` attribute or " +
					"this map, never both. Retention must be given as `log.retention.ms` " +
					"(the `log.retention.minutes` / `log.retention.hours` aliases are not accepted). " +
					"Values must be written in their canonical string form or Terraform will show " +
					"drift on the next plan.",
				Optional:    true,
				ElementType: types.StringType,
			},
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
		Events:              plan.Events,
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
		return
	}

	r.applyEvents(ctx, state, &resp.State, &resp.Diagnostics)
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

	// Preserve the original default_topic_type value if it was null
	var originalConfig models.VirtualClusterConfiguration
	var hadNullDefaultTopicType bool
	if !state.Configuration.IsNull() {
		diags = state.Configuration.As(ctx, &originalConfig, basetypes.ObjectAsOptions{})
		if !diags.HasError() {
			hadNullDefaultTopicType = originalConfig.DefaultTopicType.IsNull()
		}
	}

	r.readConfiguration(ctx, *cluster, state.BrokerConfiguration, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Whether default_topic_type is managed via broker_configuration; if so we let the
	// API-provided value stand rather than forcing it back to null.
	brokerState := brokerConfigMap(ctx, state.BrokerConfiguration, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	_, topicTypeOwnedByMap := brokerState["warpstream.default.topic.type"]

	// Preserve null value for default_topic_type if it was null in the previous state.
	// The API returns "classic" as the default, but we want to keep it as null in the
	// Terraform state to distinguish between "explicitly set to classic" and "using default".
	if hadNullDefaultTopicType && !topicTypeOwnedByMap {
		var cfgState models.VirtualClusterConfiguration
		diags = resp.State.GetAttribute(ctx, path.Root("configuration"), &cfgState)
		if !diags.HasError() {
			cfgState.DefaultTopicType = types.StringNull()
			diags = resp.State.SetAttribute(ctx, path.Root("configuration"), cfgState)
			resp.Diagnostics.Append(diags...)
		}
	}

	// Get current event types from state to filter API response.
	eventTypesFilter := types.MapNull(types.ObjectType{AttrTypes: models.EventTypeConfig{}.AttributeTypes()})
	if !state.Events.IsNull() {
		// If events is not null, get the current event types from state to use as a filter.
		var currentEvents models.VirtualClusterEvents
		diags = state.Events.As(ctx, &currentEvents, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		eventTypesFilter = currentEvents.EventTypes
	}

	r.readEvents(ctx, *cluster, &resp.State, &resp.Diagnostics, eventTypesFilter)
	r.readTags(ctx, *cluster, &resp.State, &resp.Diagnostics)
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
	if resp.Diagnostics.HasError() {
		return
	}

	// Update virtual cluster events
	r.applyEvents(ctx, plan, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

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

// filterClusterConfigsToDeclared filters the API-returned generic configs down to only
// the keys the user declared in `broker_configuration`, with the API-provided value. This
// prevents Terraform from seeing perpetual drift when the API returns configs (including
// typed-backed ones) that weren't declared. It returns a null map when nothing is declared,
// so an absent attribute round-trips to null. It mirrors filterConfigsToPlan on the topic
// resource.
func filterClusterConfigsToDeclared(ctx context.Context, apiConfigs map[string]*string, declared types.Map, respDiags *diag.Diagnostics) types.Map {
	declaredKeys := brokerConfigMap(ctx, declared, respDiags)
	if respDiags.HasError() || len(declaredKeys) == 0 {
		return types.MapNull(types.StringType)
	}

	out := make(map[string]attr.Value, len(declaredKeys))
	for k := range declaredKeys {
		if v, ok := apiConfigs[k]; ok {
			out[k] = types.StringPointerValue(v)
		}
	}
	if len(out) == 0 {
		return types.MapNull(types.StringType)
	}

	m, diags := types.MapValue(types.StringType, out)
	respDiags.Append(diags...)
	return m
}

func (r *virtualClusterResource) readConfiguration(ctx context.Context, cluster api.VirtualCluster, declared types.Map, state *tfsdk.State, respDiags *diag.Diagnostics) {
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
		ACLShadowingEnabled:      types.BoolValue(cfg.ACLShadowingEnabled),
		AutoCreateTopic:          types.BoolPointerValue(cfg.AutoCreateTopic),
		DefaultNumPartitions:     types.Int64PointerValue(cfg.DefaultNumPartitions),
		DefaultRetention:         types.Int64PointerValue(cfg.DefaultRetentionMillis),
		EnableDeletionProtection: types.BoolValue(cfg.EnableDeletionProtection),
		EnableSoftTopicDeletion:  types.BoolPointerValue(cfg.EnableSoftTopicDeletion),
	}
	if cfg.DefaultTopicType != nil {
		cfgState.DefaultTopicType = types.StringValue(*cfg.DefaultTopicType)
	} else {
		cfgState.DefaultTopicType = types.StringNull()
	}
	if cfg.SoftTopicDeletionTTL != nil {
		cfgState.SoftTopicDeletionTTL = types.Int64Value(cfg.SoftTopicDeletionTTL.Milliseconds())
	} else {
		cfgState.SoftTopicDeletionTTL = types.Int64Value(86400000)
	}

	// Set configuration state
	diags := state.SetAttribute(ctx, path.Root("configuration"), cfgState)
	respDiags.Append(diags...)

	// Set generic broker_configuration, filtered to the keys the user declared so the API's
	// full config set doesn't cause perpetual drift.
	filtered := filterClusterConfigsToDeclared(ctx, cfg.Configs, declared, respDiags)
	diags = state.SetAttribute(ctx, path.Root("broker_configuration"), filtered)
	respDiags.Append(diags...)

	// Set tier
	diags = state.SetAttribute(ctx, path.Root("tier"), types.StringValue(cfg.Tier))
	respDiags.Append(diags...)
}

func (r *virtualClusterResource) applyConfiguration(ctx context.Context, plan models.VirtualClusterResource, state *tfsdk.State, respDiags *diag.Diagnostics) {
	cluster := plan.Cluster()

	brokerCfg := brokerConfigMap(ctx, plan.BrokerConfiguration, respDiags)
	if respDiags.HasError() {
		return
	}

	// If neither the typed configuration nor the generic broker_configuration map is set,
	// just retrieve the current configuration from the API.
	if plan.Configuration.IsNull() && len(brokerCfg) == 0 {
		tflog.Info(ctx, "No virtual cluster configuration provided")
		r.readConfiguration(ctx, cluster, plan.BrokerConfiguration, state, respDiags)
		return
	}

	// owned reports whether the given underlying setting is managed via broker_configuration
	// (any of its broker keys is present). When it is, the corresponding typed field is NOT
	// sent, because the API rejects setting the same config via both a typed field and the map.
	owned := func(keys ...string) bool {
		for _, k := range keys {
			if _, ok := brokerCfg[k]; ok {
				return true
			}
		}
		return false
	}

	cfg := &api.VirtualClusterConfiguration{}

	// Retrieve typed configuration values from plan, if present.
	var cfgPlan models.VirtualClusterConfiguration
	if !plan.Configuration.IsNull() {
		diags := plan.Configuration.As(ctx, &cfgPlan, basetypes.ObjectAsOptions{})
		respDiags.Append(diags...)
		if respDiags.HasError() {
			return
		}

		// Typed fields with no generic-map equivalent are always sent.
		cfg.AclsEnabled = cfgPlan.AclsEnabled.ValueBool()
		cfg.ACLShadowingEnabled = cfgPlan.ACLShadowingEnabled.ValueBool()
		cfg.EnableDeletionProtection = cfgPlan.EnableDeletionProtection.ValueBool()

		// Typed fields with a generic-map equivalent are sent only when the setting is not
		// managed via broker_configuration.
		if !owned("auto.create.topics.enable") {
			cfg.AutoCreateTopic = cfgPlan.AutoCreateTopic.ValueBoolPointer()
		}
		if !owned("num.partitions") {
			cfg.DefaultNumPartitions = cfgPlan.DefaultNumPartitions.ValueInt64Pointer()
		}
		if !owned("log.retention.ms") {
			cfg.DefaultRetentionMillis = cfgPlan.DefaultRetention.ValueInt64Pointer()
		}
		if !owned("warpstream.soft.delete.topic.enable") {
			cfg.EnableSoftTopicDeletion = cfgPlan.EnableSoftTopicDeletion.ValueBoolPointer()
		}
		if !owned("warpstream.default.topic.type") && !cfgPlan.DefaultTopicType.IsNull() && !cfgPlan.DefaultTopicType.IsUnknown() {
			topicTypeValue := cfgPlan.DefaultTopicType.ValueString()
			cfg.DefaultTopicType = &topicTypeValue
		}
		if !owned("warpstream.soft.delete.topic.ttl.hours") && !cfgPlan.SoftTopicDeletionTTL.IsNull() && !cfgPlan.SoftTopicDeletionTTL.IsUnknown() {
			ttlValue := cfgPlan.SoftTopicDeletionTTL.ValueInt64()
			duration := time.Duration(ttlValue) * time.Millisecond
			cfg.SoftTopicDeletionTTL = &duration
		}
	}

	// Attach generic broker_configuration entries. The provider forwards these blindly; the
	// API validates the keys/values and rejects unknown ones.
	if len(brokerCfg) > 0 {
		cfg.Configs = make(map[string]*string, len(brokerCfg))
		for k, v := range brokerCfg {
			val := v
			cfg.Configs[k] = &val
		}
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
	r.readConfiguration(ctx, cluster, plan.BrokerConfiguration, state, respDiags)
	if respDiags.HasError() {
		return
	}

	// Preserve null value for default_topic_type if it wasn't explicitly set in the plan and
	// is not managed via broker_configuration. The API returns "classic" as the default, but
	// we want to keep it as null in the Terraform state to distinguish between "explicitly set
	// to classic" and "using default".
	if !owned("warpstream.default.topic.type") && (cfgPlan.DefaultTopicType.IsNull() || cfgPlan.DefaultTopicType.IsUnknown()) {
		var cfgState models.VirtualClusterConfiguration
		diags := state.GetAttribute(ctx, path.Root("configuration"), &cfgState)
		if !diags.HasError() {
			cfgState.DefaultTopicType = types.StringNull()
			diags = state.SetAttribute(ctx, path.Root("configuration"), cfgState)
			respDiags.Append(diags...)
		}
	}
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

func (r *virtualClusterResource) readEvents(ctx context.Context, cluster api.VirtualCluster, state *tfsdk.State, respDiags *diag.Diagnostics, planEventTypes types.Map) {
	// Get virtual cluster events state
	eventsState, err := r.client.GetEventsState(cluster)
	if err != nil {
		respDiags.AddError(
			"Unable to Read events state of Virtual Cluster with ID="+cluster.ID,
			err.Error(),
		)
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Events State: %+v", *eventsState))

	// Convert event types from API to Terraform model
	var eventTypesMap map[string]attr.Value
	if len(eventsState.EventTypes) > 0 && !planEventTypes.IsNull() && !planEventTypes.IsUnknown() {
		eventTypesMap = make(map[string]attr.Value)
		planElements := planEventTypes.Elements()

		for eventType, config := range eventsState.EventTypes {
			// Only include this event type if it was in the plan
			if _, inPlan := planElements[eventType]; !inPlan {
				continue
			}
			eventTypeAttrs := map[string]attr.Value{}

			if config.Enabled != nil {
				eventTypeAttrs["enabled"] = types.BoolValue(*config.Enabled)
			} else {
				eventTypeAttrs["enabled"] = types.BoolNull()
			}

			if config.RetentionPeriodNanos != nil {
				eventTypeAttrs["retention_period_nanos"] = types.Int64Value(int64(*config.RetentionPeriodNanos))
			} else {
				eventTypeAttrs["retention_period_nanos"] = types.Int64Null()
			}

			eventTypeObj, diags := types.ObjectValue(
				models.EventTypeConfig{}.AttributeTypes(),
				eventTypeAttrs,
			)
			respDiags.Append(diags...)
			if respDiags.HasError() {
				return
			}
			eventTypesMap[eventType] = eventTypeObj
		}
	}

	var eventTypesValue types.Map
	if eventTypesMap != nil {
		var diags diag.Diagnostics
		eventTypesValue, diags = types.MapValue(
			types.ObjectType{AttrTypes: models.EventTypeConfig{}.AttributeTypes()},
			eventTypesMap,
		)
		respDiags.Append(diags...)
		if respDiags.HasError() {
			return
		}
	} else {
		eventTypesValue = types.MapNull(types.ObjectType{AttrTypes: models.EventTypeConfig{}.AttributeTypes()})
	}

	eventsModel := models.VirtualClusterEvents{
		Enabled:    types.BoolValue(eventsState.Enabled),
		EventTypes: eventTypesValue,
	}

	// Set events state
	diags := state.SetAttribute(ctx, path.Root("events"), eventsModel)
	respDiags.Append(diags...)
}

func (r *virtualClusterResource) applyEvents(ctx context.Context, plan models.VirtualClusterResource, state *tfsdk.State, respDiags *diag.Diagnostics) {
	cluster := plan.Cluster()

	// If events plan is empty, just retrieve it from API
	if plan.Events.IsNull() {
		tflog.Info(ctx, "No virtual cluster events configuration provided")
		// Pass null map to read all event types from API
		r.readEvents(ctx, cluster, state, respDiags, types.MapNull(types.ObjectType{AttrTypes: models.EventTypeConfig{}.AttributeTypes()}))
		return
	}

	// Retrieve events values from plan
	var eventsPlan models.VirtualClusterEvents
	diags := plan.Events.As(ctx, &eventsPlan, basetypes.ObjectAsOptions{})
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}

	// Prepare enabled pointer
	var enabledPtr *bool
	if !eventsPlan.Enabled.IsNull() && !eventsPlan.Enabled.IsUnknown() {
		enabled := eventsPlan.Enabled.ValueBool()
		enabledPtr = &enabled
	}

	// Convert event types from Terraform model to API
	var eventTypesMap map[string]api.EventTypeConfig
	if !eventsPlan.EventTypes.IsNull() && !eventsPlan.EventTypes.IsUnknown() {
		eventTypesMap = make(map[string]api.EventTypeConfig)

		// Get the map elements
		elements := eventsPlan.EventTypes.Elements()
		for eventTypeName, eventTypeValue := range elements {
			var eventTypeConfig models.EventTypeConfig
			eventTypeObj, ok := eventTypeValue.(types.Object)
			if !ok {
				respDiags.AddError(
					"Error Converting Event Type",
					fmt.Sprintf("Expected event type %s to be an object, got %T", eventTypeName, eventTypeValue),
				)
				return
			}
			diags := eventTypeObj.As(ctx, &eventTypeConfig, basetypes.ObjectAsOptions{})
			respDiags.Append(diags...)
			if respDiags.HasError() {
				return
			}

			apiConfig := api.EventTypeConfig{}

			if !eventTypeConfig.Enabled.IsNull() && !eventTypeConfig.Enabled.IsUnknown() {
				enabled := eventTypeConfig.Enabled.ValueBool()
				apiConfig.Enabled = &enabled
			}

			if !eventTypeConfig.RetentionPeriodNanos.IsNull() && !eventTypeConfig.RetentionPeriodNanos.IsUnknown() {
				retentionPeriod := uint64(eventTypeConfig.RetentionPeriodNanos.ValueInt64())
				apiConfig.RetentionPeriodNanos = &retentionPeriod
			}

			eventTypesMap[eventTypeName] = apiConfig
		}
	}

	// Update virtual cluster events state
	err := r.client.UpdateEventsState(enabledPtr, eventTypesMap, cluster)
	if err != nil {
		respDiags.AddError(
			"Error Updating WarpStream Virtual Cluster Events State",
			"Could not update WarpStream Virtual Cluster Events State, unexpected error: "+err.Error(),
		)
		return
	}

	// Retrieve updated virtual cluster events state, filtering to only the event types in the plan
	r.readEvents(ctx, cluster, state, respDiags, eventsPlan.EventTypes)
}
