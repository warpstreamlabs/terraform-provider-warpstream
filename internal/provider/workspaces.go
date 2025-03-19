package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

type workspaceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ApplicationKey types.Object `tfsdk:"application_key"`
	CreatedAt      types.String `tfsdk:"created_at"`
}
