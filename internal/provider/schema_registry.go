package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type schemaRegistryDataSourceModel struct {
	ID           types.String     `tfsdk:"id"`
	Name         types.String     `tfsdk:"name"`
	AgentKeys    *[]agentKeyModel `tfsdk:"agent_keys"`
	CreatedAt    types.String     `tfsdk:"created_at"`
	Cloud        types.Object     `tfsdk:"cloud"`
	BootstrapURL types.String     `tfsdk:"bootstrap_url"`
	WorkspaceID  types.String     `tfsdk:"workspace_id"`
}

type schemaRegistryResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	AgentKeys    types.List   `tfsdk:"agent_keys"`
	CreatedAt    types.String `tfsdk:"created_at"`
	Cloud        types.Object `tfsdk:"cloud"`
	BootstrapURL types.String `tfsdk:"bootstrap_url"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
}

type virtualClusterRegistryCloudModel struct {
	Provider types.String `tfsdk:"provider"`
	Region   types.String `tfsdk:"region"`
}

func (m virtualClusterRegistryCloudModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"provider": types.StringType,
		"region":   types.StringType,
	}
}

func (m virtualClusterRegistryCloudModel) DefaultObject() map[string]attr.Value {
	return map[string]attr.Value{
		"provider": types.StringValue("aws"),
		"region":   types.StringValue("us-east-1"),
	}
}
