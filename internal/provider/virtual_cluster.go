package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// virtualClusterModel maps virtual cluster schema data.
type virtualClusterModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	AgentPoolID   types.String `tfsdk:"agent_pool_id"`
	AgentPoolName types.String `tfsdk:"agent_pool_name"`
	CreatedAt     types.String `tfsdk:"created_at"`
	Default       types.Bool   `tfsdk:"default"`
	Configuration types.Object `tfsdk:"configuration"`
}

type virtualClusterConfigurationModel struct {
	AclsEnabled types.Bool `tfsdk:"enable_acls"`
}

func (m virtualClusterConfigurationModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enable_acls": types.BoolType,
	}
}
