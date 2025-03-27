package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

type workspaceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type workspaceDataSourceModel struct {
	workspaceModel
	ApplicationKeys []applicationKeyModel `tfsdk:"application_keys"`
}
