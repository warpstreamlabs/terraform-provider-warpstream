package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type schemaRegistryDataSourceModel struct {
	ID            types.String     `tfsdk:"id"`
	Name          types.String     `tfsdk:"name"`
	AgentKeys     *[]agentKeyModel `tfsdk:"agent_keys"`
	AgentPoolID   types.String     `tfsdk:"agent_pool_id"`
	AgentPoolName types.String     `tfsdk:"agent_pool_name"`
	CreatedAt     types.String     `tfsdk:"created_at"`
	Cloud         types.Object     `tfsdk:"cloud"`
	BootstrapURL  types.String     `tfsdk:"bootstrap_url"`
	Configuration types.Object     `tfsdk:"configuration"`
}

type schemaRegistryResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	AgentKeys     types.List   `tfsdk:"agent_keys"`
	AgentPoolID   types.String `tfsdk:"agent_pool_id"`
	AgentPoolName types.String `tfsdk:"agent_pool_name"`
	CreatedAt     types.String `tfsdk:"created_at"`
	Cloud         types.Object `tfsdk:"cloud"`
	BootstrapURL  types.String `tfsdk:"bootstrap_url"`
	Configuration types.Object `tfsdk:"configuration"`
}

// TODO (schemaregistry): We can use this to set schema registry global configs.
type schemaRegistryConfigurationModel struct{}

func (m schemaRegistryConfigurationModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{}
}

func (m schemaRegistryConfigurationModel) DefaultObject() map[string]attr.Value {
	return map[string]attr.Value{}
}
