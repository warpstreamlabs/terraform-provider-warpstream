package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type WorkspaceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type WorkspaceDataSourceModel struct {
	WorkspaceModel
	ApplicationKeys []ApplicationKeyModel `tfsdk:"application_keys"`
}
