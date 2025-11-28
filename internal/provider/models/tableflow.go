package models

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type TableFlowDataSource struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Tier        types.String `tfsdk:"tier"`
	AgentKeys   *[]AgentKey  `tfsdk:"agent_keys"`
	CreatedAt   types.String `tfsdk:"created_at"`
	Cloud       types.Object `tfsdk:"cloud"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

type TableFlowResource struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Tier        types.String `tfsdk:"tier"`
	AgentKeys   types.List   `tfsdk:"agent_keys"`
	CreatedAt   types.String `tfsdk:"created_at"`
	Cloud       types.Object `tfsdk:"cloud"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}
