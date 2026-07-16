package models

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// VirtualClusterDataSource maps virtual cluster schema data.
type VirtualClusterDataSource struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	Tier          types.String `tfsdk:"tier"`
	AgentKeys     *[]AgentKey  `tfsdk:"agent_keys"`
	AgentPoolID   types.String `tfsdk:"agent_pool_id"`
	AgentPoolName types.String `tfsdk:"agent_pool_name"`
	CreatedAt     types.String `tfsdk:"created_at"`
	Default       types.Bool   `tfsdk:"default"`
	Tags          types.Map    `tfsdk:"tags"`
	Configuration types.Object `tfsdk:"configuration"`
	Events        types.Object `tfsdk:"events"`
	Cloud         types.Object `tfsdk:"cloud"`
	BootstrapURL  types.String `tfsdk:"bootstrap_url"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
}

type VirtualClusterResource struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	Tier          types.String `tfsdk:"tier"`
	AgentPoolID   types.String `tfsdk:"agent_pool_id"`
	AgentPoolName types.String `tfsdk:"agent_pool_name"`
	CreatedAt     types.String `tfsdk:"created_at"`
	Default       types.Bool   `tfsdk:"default"`
	Tags          types.Map    `tfsdk:"tags"`
	Configuration types.Object `tfsdk:"configuration"`
	// BrokerConfiguration is a generic map of Kafka-style broker/cluster config
	// (e.g. "message.max.bytes") for settings that don't have a dedicated typed
	// attribute under `configuration`, and for settings the user prefers to manage
	// generically. A given setting may be set via the typed attribute or this map,
	// never both.
	BrokerConfiguration types.Map    `tfsdk:"broker_configuration"`
	Events              types.Object `tfsdk:"events"`
	Cloud               types.Object `tfsdk:"cloud"`
	BootstrapURL        types.String `tfsdk:"bootstrap_url"`
	WorkspaceID         types.String `tfsdk:"workspace_id"`
}

func (m VirtualClusterResource) Cluster() api.VirtualCluster {
	var burl *string
	if m.BootstrapURL.ValueString() != "" {
		burlStr := m.BootstrapURL.ValueString()
		burl = &burlStr
	}

	return api.VirtualCluster{
		ID:            m.ID.ValueString(),
		Name:          m.Name.ValueString(),
		Type:          m.Type.ValueString(),
		AgentPoolID:   m.AgentPoolID.ValueString(),
		AgentPoolName: m.AgentPoolName.ValueString(),
		CreatedAt:     m.CreatedAt.ValueString(),
		BootstrapURL:  burl,
	}
}

type VirtualClusterConfiguration struct {
	AclsEnabled              types.Bool   `tfsdk:"enable_acls"`
	ACLShadowingEnabled      types.Bool   `tfsdk:"enable_acl_shadowing"`
	AutoCreateTopic          types.Bool   `tfsdk:"auto_create_topic"`
	DefaultNumPartitions     types.Int64  `tfsdk:"default_num_partitions"`
	DefaultRetention         types.Int64  `tfsdk:"default_retention_millis"`
	DefaultTopicType         types.String `tfsdk:"default_topic_type"`
	EnableDeletionProtection types.Bool   `tfsdk:"enable_deletion_protection"`
	EnableSoftTopicDeletion  types.Bool   `tfsdk:"enable_soft_topic_deletion"`
	SoftTopicDeletionTTL     types.Int64  `tfsdk:"soft_topic_deletion_ttl_millis"`
}

func (m VirtualClusterConfiguration) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"auto_create_topic":              types.BoolType,
		"default_num_partitions":         types.Int64Type,
		"default_retention_millis":       types.Int64Type,
		"default_topic_type":             types.StringType,
		"enable_acls":                    types.BoolType,
		"enable_acl_shadowing":           types.BoolType,
		"enable_deletion_protection":     types.BoolType,
		"enable_soft_topic_deletion":     types.BoolType,
		"soft_topic_deletion_ttl_millis": types.Int64Type,
	}
}

func (m VirtualClusterConfiguration) DefaultObject() map[string]attr.Value {
	return map[string]attr.Value{
		"auto_create_topic":              types.BoolValue(true),
		"default_num_partitions":         types.Int64Value(1),
		"default_retention_millis":       types.Int64Value(86400000),
		"default_topic_type":             types.StringNull(),
		"enable_acls":                    types.BoolValue(false),
		"enable_acl_shadowing":           types.BoolValue(false),
		"enable_deletion_protection":     types.BoolValue(false),
		"enable_soft_topic_deletion":     types.BoolValue(true),
		"soft_topic_deletion_ttl_millis": types.Int64Value(86400000),
	}
}

type VirtualClusterCloud struct {
	Provider    types.String `tfsdk:"provider"`
	Region      types.String `tfsdk:"region"`
	RegionGroup types.String `tfsdk:"region_group"`
}

func (m VirtualClusterCloud) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"provider":     types.StringType,
		"region":       types.StringType,
		"region_group": types.StringType,
	}
}

func (m VirtualClusterCloud) DefaultObject() map[string]attr.Value {
	return map[string]attr.Value{
		"provider":     types.StringValue("aws"),
		"region":       types.StringValue("us-east-1"),
		"region_group": types.StringNull(),
	}
}

// EventTypeConfig represents per-event-type configuration.
type EventTypeConfig struct {
	Enabled              types.Bool  `tfsdk:"enabled"`
	RetentionPeriodNanos types.Int64 `tfsdk:"retention_period_nanos"`
}

func (m EventTypeConfig) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":                types.BoolType,
		"retention_period_nanos": types.Int64Type,
	}
}

// VirtualClusterEvents represents the events configuration for a virtual cluster.
type VirtualClusterEvents struct {
	Enabled    types.Bool `tfsdk:"enabled"`
	EventTypes types.Map  `tfsdk:"event_types"`
}

func (m VirtualClusterEvents) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled": types.BoolType,
		"event_types": types.MapType{
			ElemType: types.ObjectType{
				AttrTypes: EventTypeConfig{}.AttributeTypes(),
			},
		},
	}
}

func (m VirtualClusterEvents) DefaultObject() map[string]attr.Value {
	return map[string]attr.Value{
		"enabled": types.BoolValue(false),
		"event_types": types.MapNull(types.ObjectType{
			AttrTypes: EventTypeConfig{}.AttributeTypes(),
		}),
	}
}
