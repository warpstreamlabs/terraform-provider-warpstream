package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type Workspace struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type WorkspaceDataSource struct {
	Workspace
	ApplicationKeys []ApplicationKey `tfsdk:"application_keys"`
}
