package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// virtualClusterModel maps virtual cluster schema data.
type virtualClusterDataSourceModel struct {
	ID            types.String     `tfsdk:"id"`
	Name          types.String     `tfsdk:"name"`
	Type          types.String     `tfsdk:"type"`
	Tier          types.String     `tfsdk:"tier"`
	AgentKeys     *[]agentKeyModel `tfsdk:"agent_keys"`
	AgentPoolID   types.String     `tfsdk:"agent_pool_id"`
	AgentPoolName types.String     `tfsdk:"agent_pool_name"`
	CreatedAt     types.String     `tfsdk:"created_at"`
	Default       types.Bool       `tfsdk:"default"`
	Tags          types.Map        `tfsdk:"tags"`
	Configuration types.Object     `tfsdk:"configuration"`
	Cloud         types.Object     `tfsdk:"cloud"`
	BootstrapURL  types.String     `tfsdk:"bootstrap_url"`
	WorkspaceID   types.String     `tfsdk:"workspace_id"`
}

type virtualClusterResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Type          types.String `tfsdk:"type"`
	Tier          types.String `tfsdk:"tier"`
	AgentKeys     types.List   `tfsdk:"agent_keys"`
	AgentPoolID   types.String `tfsdk:"agent_pool_id"`
	AgentPoolName types.String `tfsdk:"agent_pool_name"`
	CreatedAt     types.String `tfsdk:"created_at"`
	Default       types.Bool   `tfsdk:"default"`
	Tags          types.Map    `tfsdk:"tags"`
	Configuration types.Object `tfsdk:"configuration"`
	Cloud         types.Object `tfsdk:"cloud"`
	BootstrapURL  types.String `tfsdk:"bootstrap_url"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
}

type virtualClusterConfigurationModel struct {
	AclsEnabled              types.Bool  `tfsdk:"enable_acls"`
	AutoCreateTopic          types.Bool  `tfsdk:"auto_create_topic"`
	DefaultNumPartitions     types.Int64 `tfsdk:"default_num_partitions"`
	DefaultRetention         types.Int64 `tfsdk:"default_retention_millis"`
	EnableDeletionProtection types.Bool  `tfsdk:"enable_deletion_protection"`
}

func (m virtualClusterConfigurationModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"auto_create_topic":          types.BoolType,
		"default_num_partitions":     types.Int64Type,
		"default_retention_millis":   types.Int64Type,
		"enable_acls":                types.BoolType,
		"enable_deletion_protection": types.BoolType,
	}
}

func (m virtualClusterConfigurationModel) DefaultObject() map[string]attr.Value {
	return map[string]attr.Value{
		"auto_create_topic":          types.BoolValue(true),
		"default_num_partitions":     types.Int64Value(1),
		"default_retention_millis":   types.Int64Value(86400000),
		"enable_acls":                types.BoolValue(false),
		"enable_deletion_protection": types.BoolValue(false),
	}
}

type virtualClusterCloudModel struct {
	Provider    types.String `tfsdk:"provider"`
	Region      types.String `tfsdk:"region"`
	RegionGroup types.String `tfsdk:"region_group"`
}

func (m virtualClusterCloudModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"provider":     types.StringType,
		"region":       types.StringType,
		"region_group": types.StringType,
	}
}

func (m virtualClusterCloudModel) DefaultObject() map[string]attr.Value {
	return map[string]attr.Value{
		"provider":     types.StringValue("aws"),
		"region":       types.StringValue("us-east-1"),
		"region_group": types.StringNull(),
	}
}
