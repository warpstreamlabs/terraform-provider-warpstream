package resources

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

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

// brokerConfigOverride is a `broker_configuration` entry that mirrors a typed
// `configuration` attribute, and therefore dictates that attribute's planned value.
type brokerConfigOverride struct {
	// Key is the canonical broker config name the value was declared under.
	Key string
	// Value is the declared value. It is unknown when it derives from something that will
	// not exist until apply.
	Value types.String
}

// ModifyPlan validates the generic `broker_configuration` map and reconciles it with the
// typed `configuration` attribute. It:
//
//  1. rejects unsupported config names, write-only aliases, null values, and values that are
//     not already in the canonical form describe reports back;
//  2. rejects setting one cluster setting through both a typed attribute and the map with
//     values that disagree, the one case the provider cannot resolve on the user's behalf;
//  3. rewrites each mirrored typed attribute to the value declared in the map.
//
// Step 3 is what keeps an apply consistent. `configuration` carries a static default object,
// so Terraform plans a value for every typed attribute even when the user wrote only the map.
// Left alone, the apply would write the map's value, read it back, and Terraform would abort
// because the result disagrees with the plan it approved.
func (r *virtualClusterResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Nothing to reconcile on destroy.
	if req.Plan.Raw.IsNull() {
		return
	}

	var declared types.Map
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("broker_configuration"), &declared)...)
	if resp.Diagnostics.HasError() {
		return
	}
	entries := brokerConfigEntries(ctx, declared, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || len(entries) == 0 {
		return
	}

	// Report problems in a stable order so a configuration with several mistakes does not
	// produce differently ordered output between runs.
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	brokerPath := path.Root("broker_configuration")
	for _, key := range keys {
		if err := validateBrokerConfigKey(key); err != nil {
			resp.Diagnostics.AddAttributeError(brokerPath.AtMapKey(key), "Invalid broker configuration", err.Error())
			continue
		}
		value := entries[key]
		if value.IsNull() {
			resp.Diagnostics.AddAttributeError(brokerPath.AtMapKey(key), "Invalid broker configuration",
				"null is not a valid value: the API ignores null entries, so Terraform would not be able to track this "+
					"setting. Remove the key, or set it to the value you want.")
			continue
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the typed configuration from the raw *config* rather than the plan, so that the
	// schema's defaults are not mistaken for values the user wrote.
	var cfgObj types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("configuration"), &cfgObj)...)
	if resp.Diagnostics.HasError() {
		return
	}
	declaredTypedAttrs := make(map[string]attr.Value)
	if !cfgObj.IsNull() && !cfgObj.IsUnknown() {
		for name, v := range cfgObj.Attributes() {
			if v != nil && !v.IsNull() && !v.IsUnknown() {
				declaredTypedAttrs[name] = v
			}
		}
	}

	overrides, conflicts, err := resolveBrokerConfigOverrides(entries, declaredTypedAttrs)
	if err != nil {
		resp.Diagnostics.AddError("Invalid broker configuration", err.Error())
		return
	}
	for _, c := range conflicts {
		resp.Diagnostics.AddError(
			"Conflicting virtual cluster configuration",
			fmt.Sprintf(
				"`configuration.%s` is set to %s but `broker_configuration[%q]` is set to %q, and both "+
					"control the same cluster setting. Set them to the same value, or set only one of them.",
				c.TypedAttr, c.TypedValue, c.Key, c.MapValue,
			),
		)
	}
	if resp.Diagnostics.HasError() || len(overrides) == 0 {
		return
	}

	r.reconcileTypedConfiguration(ctx, req, resp, overrides, declaredTypedAttrs)
	r.preserveNullEventTypes(ctx, req, resp)
}

// preserveNullEventTypes undoes collateral damage caused by reconciling `configuration`.
//
// When the map owns a setting whose typed attribute carries a schema default, that default is
// applied before this ModifyPlan runs, so the plan briefly disagrees with prior state. That is
// enough for the framework to mark every computed attribute that is null in state as
// known-after-apply, which catches `events.event_types` — and its UseStateForUnknown cannot
// restore it precisely because prior state is null. Reconciling `configuration` afterwards
// fixes the attribute that caused the mismatch, but not the ones swept up along the way, so an
// otherwise idempotent plan reports an events update that will never happen.
//
// The restore is deliberately narrow: only an unknown planned value, with no events declared
// in the configuration and none in prior state, is reset. That is exactly what
// UseStateForUnknown would have done had it handled a null prior state, so no other events
// behaviour changes.
func (r *virtualClusterResource) preserveNullEventTypes(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// On create there is no prior state to preserve, and known-after-apply is correct.
	if req.State.Raw.IsNull() {
		return
	}

	eventTypesPath := path.Root("events").AtName("event_types")

	var planEventTypes types.Map
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, eventTypesPath, &planEventTypes)...)
	if resp.Diagnostics.HasError() || !planEventTypes.IsUnknown() {
		return
	}

	var configEventTypes types.Map
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, eventTypesPath, &configEventTypes)...)
	if resp.Diagnostics.HasError() || !configEventTypes.IsNull() {
		return
	}

	var stateEventTypes types.Map
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, eventTypesPath, &stateEventTypes)...)
	if resp.Diagnostics.HasError() || !stateEventTypes.IsNull() {
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, eventTypesPath, stateEventTypes)...)
}

// brokerConfigConflict is a cluster setting written through both the typed `configuration`
// attribute and the `broker_configuration` map, with values that disagree.
type brokerConfigConflict struct {
	Key        string
	TypedAttr  string
	MapValue   string
	TypedValue attr.Value
}

// resolveBrokerConfigOverrides pairs every `broker_configuration` entry that mirrors a typed
// `configuration` attribute with that attribute, and reports the settings written through
// both surfaces with values that disagree. Entries whose value is not known until apply are
// still returned as overrides — the mirrored attribute has to plan as known-after-apply — but
// cannot be compared, so they never conflict.
//
// A value the API normalises on write is not detected here; checkDeclaredConfigsApplied reports
// it after the apply, so no knowledge of any config's canonical form is needed.
func resolveBrokerConfigOverrides(
	entries map[string]types.String,
	declaredTypedAttrs map[string]attr.Value,
) (map[string]brokerConfigOverride, []brokerConfigConflict, error) {
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	overrides := make(map[string]brokerConfigOverride)
	var conflicts []brokerConfigConflict
	for _, key := range keys {
		typedAttr, mirrored := brokerKeyTypedAttr[key]
		if !mirrored {
			continue
		}
		value := entries[key]
		overrides[typedAttr] = brokerConfigOverride{Key: key, Value: value}

		typedVal, alsoDeclared := declaredTypedAttrs[typedAttr]
		if !alsoDeclared || value.IsNull() || value.IsUnknown() {
			continue
		}
		// Compare in the typed attribute's own type, so "3600000" and 3600000 count as equal.
		// The type comes from the value the user wrote, not from any knowledge of the config.
		mapVal, err := brokerConfigValueAs(value.ValueString(), typedVal)
		if err != nil {
			return nil, nil, err
		}
		if !mapVal.Equal(typedVal) {
			conflicts = append(conflicts, brokerConfigConflict{
				Key:        key,
				TypedAttr:  typedAttr,
				MapValue:   value.ValueString(),
				TypedValue: typedVal,
			})
		}
	}
	return overrides, conflicts, nil
}

// brokerConfigValueAs parses raw into the same Terraform type as tmpl, so a value declared in
// `broker_configuration` can be compared with the typed `configuration` attribute that mirrors
// it. The type is taken from this provider's own schema rather than from any table describing
// the config, which is what lets the map stay generic.
func brokerConfigValueAs(raw string, tmpl attr.Value) (attr.Value, error) {
	switch tmpl.(type) {
	case types.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("%q is not a boolean", raw)
		}
		return types.BoolValue(b), nil
	case types.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", raw)
		}
		return types.Int64Value(n), nil
	case types.String:
		return types.StringValue(raw), nil
	}
	return nil, fmt.Errorf("cannot interpret %q as %T", raw, tmpl)
}

// reconcileTypedConfiguration rewrites the planned `configuration` object so every attribute
// holds the value the apply will actually produce:
//
//   - an attribute mirrored by a map entry takes the map's value, or plans as
//     known-after-apply when that value is not known until apply;
//   - an attribute the user wrote in the configuration keeps its planned value;
//   - anything else falls back to prior state, so a schema default cannot overwrite a value
//     the server already holds.
//
// On an unchanged re-plan the result equals prior state, so no update is planned and sibling
// computed attributes are not flipped to known-after-apply.
func (r *virtualClusterResource) reconcileTypedConfiguration(
	ctx context.Context,
	req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse,
	overrides map[string]brokerConfigOverride,
	declaredTypedAttrs map[string]attr.Value,
) {
	var planCfg types.Object
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("configuration"), &planCfg)...)
	if resp.Diagnostics.HasError() || planCfg.IsNull() || planCfg.IsUnknown() {
		return
	}

	// Prior state is absent on create, where there is nothing to fall back to and the
	// planned defaults stand.
	var stateAttrs map[string]attr.Value
	var stateBroker map[string]string
	if !req.State.Raw.IsNull() {
		var stateCfg types.Object
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("configuration"), &stateCfg)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !stateCfg.IsNull() && !stateCfg.IsUnknown() {
			stateAttrs = stateCfg.Attributes()
		}

		var stateMap types.Map
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("broker_configuration"), &stateMap)...)
		if resp.Diagnostics.HasError() {
			return
		}
		stateBroker = brokerConfigMap(ctx, stateMap, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	attrs := make(map[string]attr.Value, len(planCfg.Attributes()))
	for name, planVal := range planCfg.Attributes() {
		// An attribute the user wrote keeps its planned value, which is that written value. This
		// has to be checked before mirroring: Terraform rejects a plan that marks an attribute
		// unknown when the configuration gives it a value, and a setting written through both
		// surfaces has already been checked for agreement, so the two say the same thing anyway.
		if _, wasDeclared := declaredTypedAttrs[name]; wasDeclared {
			attrs[name] = planVal
			continue
		}
		if override, mirrored := overrides[name]; mirrored {
			attrs[name] = plannedMirroredValue(override, planVal, stateAttrs[name], stateBroker)
			continue
		}
		if stateVal, ok := stateAttrs[name]; ok {
			attrs[name] = stateVal
			continue
		}
		attrs[name] = planVal
	}

	newObj, diags := types.ObjectValue(planCfg.AttributeTypes(ctx), attrs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("configuration"), newObj)...)
}

// plannedMirroredValue returns the value a typed `configuration` attribute must hold in the plan
// when a `broker_configuration` entry controls the same underlying setting.
//
// The provider deliberately does not try to predict what the API will report. Values are
// normalised server-side in ways it has no general knowledge of — any negative retention becomes
// "-1", a soft-delete TTL becomes a duration clamped to 100 years — and encoding those rules here
// is exactly the sort of API knowledge that goes stale the moment a config is added. Instead:
//
//   - if the declaration has not changed since the last apply, prior state already holds whatever
//     the API reported for it, so reuse that and the plan settles with no diff;
//   - otherwise plan known-after-apply and let the read that follows the write supply the value.
//
// That is correct for any normalisation, including ones the provider has never heard of.
func plannedMirroredValue(override brokerConfigOverride, planVal, priorVal attr.Value, priorBroker map[string]string) attr.Value {
	declaredUnchanged := !override.Value.IsNull() && !override.Value.IsUnknown() &&
		priorBroker[override.Key] == override.Value.ValueString()

	if declaredUnchanged && priorVal != nil && !priorVal.IsNull() && !priorVal.IsUnknown() {
		return priorVal
	}
	return unknownLike(planVal)
}

// unknownLike returns the unknown value of the same Terraform type as v. The typed
// `configuration` attributes are all booleans, 64-bit integers or strings; anything else is
// returned unchanged rather than guessed at.
func unknownLike(v attr.Value) attr.Value {
	switch v.(type) {
	case types.Bool:
		return types.BoolUnknown()
	case types.Int64:
		return types.Int64Unknown()
	case types.String:
		return types.StringUnknown()
	}
	return v
}

// brokerConfigEntries extracts a `broker_configuration` map into Go, keeping each value as a
// types.String so that entries which are not known until apply survive the conversion. It
// returns nil when the attribute as a whole is null or unknown.
func brokerConfigEntries(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]types.String {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := make(map[string]types.String, len(m.Elements()))
	diags.Append(m.ElementsAs(ctx, &out, false)...)
	return out
}

// brokerConfigMap extracts the known entries of a `broker_configuration` map into a plain Go
// map, for the write path where every value has been resolved. Null and not-yet-known
// entries are skipped: the API treats a null entry as absent, and by the time configuration
// is written there is nothing left unknown.
func brokerConfigMap(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	entries := brokerConfigEntries(ctx, m, diags)
	if diags.HasError() || len(entries) == 0 {
		return nil
	}
	out := make(map[string]string, len(entries))
	for k, v := range entries {
		if v.IsNull() || v.IsUnknown() {
			continue
		}
		out[k] = v.ValueString()
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
						Description:        "Enable topic autocreation feature, defaults to `true`.",
						DeprecationMessage: "Set this via `broker_configuration` (key `auto.create.topics.enable`) instead, which is the canonical way to configure broker settings.",
						Optional:           true,
						Computed:           true,
						Default:            booldefault.StaticBool(true),
					},
					"default_num_partitions": schema.Int64Attribute{
						Description:        "Number of partitions created by default.",
						DeprecationMessage: "Set this via `broker_configuration` (key `num.partitions`) instead, which is the canonical way to configure broker settings.",
						Optional:           true,
						Computed:           true,
						Default:            int64default.StaticInt64(1),
					},
					"default_retention_millis": schema.Int64Attribute{
						Description:        "Default retention for topics that are created automatically using Kafka's topic auto-creation feature.",
						DeprecationMessage: "Set this via `broker_configuration` (key `log.retention.ms`) instead, which is the canonical way to configure broker settings.",
						Optional:           true,
						Computed:           true,
						Default:            int64default.StaticInt64(86400000),
					},
					"default_topic_type": schema.StringAttribute{
						Description:        "Default topic type for new topics. Valid values are `classic` or `lightning`. If not specified, the WarpStream API defaults to `classic`. See [Lightning Topics](https://docs.warpstream.com/warpstream/kafka/advanced-agent-deployment-options/low-latency-clusters/lightning-topics)",
						DeprecationMessage: "Set this via `broker_configuration` (key `warpstream.default.topic.type`) instead, which is the canonical way to configure broker settings.",
						Optional:           true,
						Computed:           true,
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
						Description:        "Enable soft deletion for topics. Defaults to `true`. If true, topic deletion will be a soft deletion. For clusters with the Fundamentals tier or above, it will be possible to restore topics for some time after deletion. If false, deleting a topic will immediately delete of all of its data, with no way to recover it.",
						DeprecationMessage: "Set this via `broker_configuration` (key `warpstream.soft.delete.topic.enable`) instead, which is the canonical way to configure broker settings.",
						Optional:           true,
						Computed:           true,
						Default:            booldefault.StaticBool(true),
					},
					"soft_topic_deletion_ttl_millis": schema.Int64Attribute{
						Description:        "If enable_soft_topic_deletion is true, a deleted topic's data will be kept for this many milliseconds before being irrecoverably deleted. Defaults to 24 hours.",
						DeprecationMessage: "Set this via `broker_configuration` (key `warpstream.soft.delete.topic.ttl.ms`) instead, which is the canonical way to configure broker settings.",
						Optional:           true,
						Computed:           true,
						Default:            int64default.StaticInt64(86400000),
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
				// Kept to a single paragraph: tfplugindocs renders this inline in the attribute
				// list, and a blank line would end the list and orphan every attribute after it.
				Description: "Cluster-level broker configuration, as a map of Kafka-style config names to " +
					"string values (e.g. `message.max.bytes = \"1048576\"`, `delete.topic.enable = \"true\"`). " +
					"This is the canonical, recommended way to configure broker settings; the individual " +
					"typed attributes under `configuration` (such as `default_retention_millis` and " +
					"`default_topic_type`) are deprecated in favor of the equivalent key here. " +
					"A setting that also has a typed `configuration` attribute may be set through either " +
					"surface, or through both as long as the two values agree; setting them to different " +
					"values is rejected at plan time. " +
					"Any config name the API supports may be used here; the provider does not keep its own " +
					"list, so a config added to the API is usable without a provider upgrade. " +
					"Values must be written exactly as the API reports them back, because Terraform " +
					"compares the value in state against the one in your configuration. Where the API " +
					"rewrites a value, the apply fails with the form to use instead, so for example " +
					"write `true` rather than `T`, `lightning` rather than `Lightning`, and `-1` for " +
					"infinite retention rather than any other negative number. " +
					"Two config names cannot be used: specify retention as `log.retention.ms` " +
					"(not `log.retention.minutes` or `log.retention.hours`) and the soft-delete topic TTL as " +
					"`warpstream.soft.delete.topic.ttl.ms` (not `warpstream.soft.delete.topic.ttl.hours`), " +
					"because the API accepts those aliases on write but only ever reports the millisecond " +
					"form, so Terraform could not track them. " +
					"Note that removing a key from this map does **not** reset the config on the cluster: " +
					"the WarpStream API has no way to revert a config to its default, so an omitted config " +
					"keeps whatever value it already had. To change a setting back, set it explicitly to " +
					"the value you want.",
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
		// The cluster exists but could not be configured, most often because the API rejected a
		// broker config. Terraform refuses any state still holding unknown values and reports
		// that as "provider returned invalid result object ... always a bug in the provider",
		// which would bury the real error under two misleading ones. Fill in what the cluster
		// actually has so the API's message is the only thing the user sees, and so the next
		// apply starts from an accurate record.
		r.readConfiguration(ctx, *cluster, state.BrokerConfiguration, &resp.State, &resp.Diagnostics)
		r.readEvents(ctx, *cluster, &resp.State, &resp.Diagnostics,
			types.MapNull(types.ObjectType{AttrTypes: models.EventTypeConfig{}.AttributeTypes()}))
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

// checkDeclaredConfigsApplied compares the broker configs the user declared against what the
// API reported once they had been written, and reports a clear error when they differ.
//
// This only matters on the apply path. `broker_configuration` is not a Computed attribute, so
// Terraform requires the value in state after an apply to equal the value in the
// configuration, and state is populated from the API's response. Terraform performs the same
// check itself and aborts with a generic "Provider produced inconsistent result after apply";
// catching it here lets us name the key and the value to write instead. On a refresh a
// difference is ordinary drift, not an error, so this is deliberately not called from Read.
func checkDeclaredConfigsApplied(declared map[string]string, apiConfigs map[string]*string, respDiags *diag.Diagnostics) {
	keys := make([]string, 0, len(declared))
	for k := range declared {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	brokerPath := path.Root("broker_configuration")
	for _, key := range keys {
		declaredValue := declared[key]
		apiValue, ok := apiConfigs[key]
		if !ok || apiValue == nil {
			respDiags.AddAttributeError(
				brokerPath.AtMapKey(key),
				"Broker configuration was not applied",
				fmt.Sprintf(
					"The API did not report cluster config %q after it was written, so Terraform cannot record "+
						"a value for it. This usually means the config is not settable on this cluster.",
					key,
				),
			)
			continue
		}
		if *apiValue != declaredValue {
			respDiags.AddAttributeError(
				brokerPath.AtMapKey(key),
				"Broker configuration was changed by the API",
				fmt.Sprintf(
					"Cluster config %q was written as %q but the API reports it as %q. Terraform cannot record a "+
						"value that differs from your configuration; write it as %q instead.",
					key, declaredValue, *apiValue, *apiValue,
				),
			)
		}
	}
}

// checkAPIConfigConsistency reports when the API's deprecated typed fields disagree with its
// own broker_configs map about the same setting. The two are views of one stored value, so a
// disagreement means the provider's understanding of the API has gone stale and the value it
// writes to state is a guess.
//
// Only keys present in the map are compared: absence means the cluster is on the built-in
// default, which the typed field still reports. Configs where a negative value means infinite
// are skipped when either side is negative, because the two representations encode infinity
// differently and the API compares them semantically rather than literally.
//
// This is a warning rather than an error. Nothing here breaks an apply — `broker_configuration`
// is taken from the map and `configuration` from the typed fields — but it should be reported.
func checkAPIConfigConsistency(cfg *api.VirtualClusterConfiguration, respDiags *diag.Diagnostics) {
	if len(cfg.BrokerConfigs) == 0 {
		return
	}

	typed := apiTypedConfigValues(cfg)
	keys := make([]string, 0, len(typed))
	for k := range typed {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		mapValue, ok := cfg.BrokerConfigs[key]
		if !ok || mapValue == nil {
			continue
		}
		typedValue := typed[key]
		if typedValue == *mapValue {
			continue
		}
		// A negative value on either side is a sentinel, most often meaning infinite, and the two
		// representations are free to encode it differently: the soft-delete TTL reports "-1" in
		// the map but a duration clamped to 100 years in its typed field. Those are not the API
		// contradicting itself, so skip rather than encode which configs behave that way.
		if strings.HasPrefix(typedValue, "-") || strings.HasPrefix(*mapValue, "-") {
			continue
		}
		respDiags.AddWarning(
			"Inconsistent virtual cluster configuration from the API",
			fmt.Sprintf(
				"The API reports cluster config %q as %q in broker_configs but as %q in its deprecated typed "+
					"field, and the two are views of the same value. Please report this issue to the provider "+
					"developers.",
				key, *mapValue, typedValue,
			),
		)
	}
}

// apiTypedConfigValues renders the API's deprecated typed configuration fields as broker
// config values, keyed by canonical config name, so they can be compared with broker_configs.
// Fields the API did not populate are omitted.
func apiTypedConfigValues(cfg *api.VirtualClusterConfiguration) map[string]string {
	out := make(map[string]string, len(typedAttrBrokerKey))
	if cfg.AutoCreateTopic != nil {
		out["auto.create.topics.enable"] = strconv.FormatBool(*cfg.AutoCreateTopic)
	}
	if cfg.DefaultNumPartitions != nil {
		out["num.partitions"] = strconv.FormatInt(*cfg.DefaultNumPartitions, 10)
	}
	if cfg.DefaultRetentionMillis != nil {
		out["log.retention.ms"] = strconv.FormatInt(*cfg.DefaultRetentionMillis, 10)
	}
	if cfg.EnableSoftTopicDeletion != nil {
		out["warpstream.soft.delete.topic.enable"] = strconv.FormatBool(*cfg.EnableSoftTopicDeletion)
	}
	if cfg.DefaultTopicType != nil {
		out["warpstream.default.topic.type"] = *cfg.DefaultTopicType
	}
	if cfg.SoftTopicDeletionTTL != nil {
		out["warpstream.soft.delete.topic.ttl.ms"] = strconv.FormatInt(cfg.SoftTopicDeletionTTL.Milliseconds(), 10)
	}
	return out
}

// brokerConfigsPayload builds the generic broker_configs request body from the declared map
// plus the typed `configuration` attributes that have a canonical config name. Entries the
// user declared in the map win, so a value is never sent through both representations, which
// is what the API rejects when the two disagree.
func brokerConfigsPayload(cfgPlan *models.VirtualClusterConfiguration, brokerCfg map[string]string) map[string]*string {
	out := make(map[string]*string, len(brokerCfg)+6)
	for k, v := range brokerCfg {
		out[k] = &v
	}

	if cfgPlan != nil {
		set := func(key, value string) {
			if _, ok := out[key]; !ok {
				out[key] = &value
			}
		}
		if !cfgPlan.AutoCreateTopic.IsNull() && !cfgPlan.AutoCreateTopic.IsUnknown() {
			set("auto.create.topics.enable", strconv.FormatBool(cfgPlan.AutoCreateTopic.ValueBool()))
		}
		if !cfgPlan.DefaultNumPartitions.IsNull() && !cfgPlan.DefaultNumPartitions.IsUnknown() {
			set("num.partitions", strconv.FormatInt(cfgPlan.DefaultNumPartitions.ValueInt64(), 10))
		}
		if !cfgPlan.DefaultRetention.IsNull() && !cfgPlan.DefaultRetention.IsUnknown() {
			set("log.retention.ms", strconv.FormatInt(cfgPlan.DefaultRetention.ValueInt64(), 10))
		}
		if !cfgPlan.EnableSoftTopicDeletion.IsNull() && !cfgPlan.EnableSoftTopicDeletion.IsUnknown() {
			set("warpstream.soft.delete.topic.enable", strconv.FormatBool(cfgPlan.EnableSoftTopicDeletion.ValueBool()))
		}
		if !cfgPlan.DefaultTopicType.IsNull() && !cfgPlan.DefaultTopicType.IsUnknown() {
			set("warpstream.default.topic.type", cfgPlan.DefaultTopicType.ValueString())
		}
		if !cfgPlan.SoftTopicDeletionTTL.IsNull() && !cfgPlan.SoftTopicDeletionTTL.IsUnknown() {
			set("warpstream.soft.delete.topic.ttl.ms", strconv.FormatInt(cfgPlan.SoftTopicDeletionTTL.ValueInt64(), 10))
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// readConfiguration writes the cluster's configuration from the API into state, and returns
// the API's response so a caller on the apply path can check what was actually stored. It
// returns nil if the configuration could not be read.
func (r *virtualClusterResource) readConfiguration(ctx context.Context, cluster api.VirtualCluster, declared types.Map, state *tfsdk.State, respDiags *diag.Diagnostics) *api.VirtualClusterConfiguration {
	// Get virtual cluster configuration
	cfg, err := r.client.GetConfiguration(cluster)
	if err != nil {
		respDiags.AddError(
			"Unable to Read configuration of Virtual Cluster with ID="+cluster.ID,
			err.Error(),
		)
		return nil
	}
	tflog.Debug(ctx, fmt.Sprintf("Configuration: %+v", *cfg))

	checkAPIConfigConsistency(cfg, respDiags)

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
	filtered := filterClusterConfigsToDeclared(ctx, cfg.BrokerConfigs, declared, respDiags)
	diags = state.SetAttribute(ctx, path.Root("broker_configuration"), filtered)
	respDiags.Append(diags...)

	// Set tier
	diags = state.SetAttribute(ctx, path.Root("tier"), types.StringValue(cfg.Tier))
	respDiags.Append(diags...)

	return cfg
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

	cfg := &api.VirtualClusterConfiguration{}

	// Retrieve typed configuration values from plan, if present.
	var cfgPlan models.VirtualClusterConfiguration
	var cfgPlanPtr *models.VirtualClusterConfiguration
	if !plan.Configuration.IsNull() {
		diags := plan.Configuration.As(ctx, &cfgPlan, basetypes.ObjectAsOptions{})
		respDiags.Append(diags...)
		if respDiags.HasError() {
			return
		}
		cfgPlanPtr = &cfgPlan

		// Only the settings with no broker_configs equivalent are sent as typed fields;
		// everything the map supports goes through broker_configs (the typed request
		// fields are deprecated).
		cfg.AclsEnabled = cfgPlan.AclsEnabled.ValueBool()
		cfg.ACLShadowingEnabled = cfgPlan.ACLShadowingEnabled.ValueBool()
		cfg.EnableDeletionProtection = cfgPlan.EnableDeletionProtection.ValueBool()
	}

	cfg.BrokerConfigs = brokerConfigsPayload(cfgPlanPtr, brokerCfg)
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
	applied := r.readConfiguration(ctx, cluster, plan.BrokerConfiguration, state, respDiags)
	if respDiags.HasError() {
		return
	}

	// Fail with a specific message if the API did not store a declared config verbatim, rather
	// than letting Terraform abort with a generic inconsistent-result error.
	if applied != nil {
		checkDeclaredConfigsApplied(brokerCfg, applied.BrokerConfigs, respDiags)
		if respDiags.HasError() {
			return
		}
	}

	// Preserve null value for default_topic_type if it wasn't explicitly set in the plan and
	// is not managed via broker_configuration. The API returns "classic" as the default, but
	// we want to keep it as null in the Terraform state to distinguish between "explicitly set
	// to classic" and "using default".
	_, topicTypeOwnedByMap := brokerCfg["warpstream.default.topic.type"]
	if !topicTypeOwnedByMap && (cfgPlan.DefaultTopicType.IsNull() || cfgPlan.DefaultTopicType.IsUnknown()) {
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
