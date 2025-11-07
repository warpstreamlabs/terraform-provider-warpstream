package models

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type TableFlowDataSource struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	AgentKeys    *[]AgentKey  `tfsdk:"agent_keys"`
	CreatedAt    types.String `tfsdk:"created_at"`
	Cloud        types.Object `tfsdk:"cloud"`
	BootstrapURL types.String `tfsdk:"bootstrap_url"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
}

type TableFlowResource struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Tier         types.String `tfsdk:"tier"`
	AgentKeys    types.List   `tfsdk:"agent_keys"`
	CreatedAt    types.String `tfsdk:"created_at"`
	Cloud        types.Object `tfsdk:"cloud"`
	BootstrapURL types.String `tfsdk:"bootstrap_url"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
}

type VirtualClusterTableFlowCloud struct {
	Provider types.String `tfsdk:"provider"`
	Region   types.String `tfsdk:"region"`
}

func (m VirtualClusterTableFlowCloud) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"provider": types.StringType,
		"region":   types.StringType,
	}
}

func (m VirtualClusterTableFlowCloud) DefaultObject() map[string]attr.Value {
	return map[string]attr.Value{
		"provider": types.StringValue("aws"),
		"region":   types.StringValue("us-east-1"),
	}
}
