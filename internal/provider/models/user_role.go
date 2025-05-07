package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type UserRole struct {
	ID           types.String    `tfsdk:"id"`
	Name         types.String    `tfsdk:"name"`
	AccessGrants []UserRoleGrant `tfsdk:"access_grants"`
	CreatedAt    types.String    `tfsdk:"created_at"`
}

type UserRoleGrant struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	GrantType   types.String `tfsdk:"grant_type"`
}
