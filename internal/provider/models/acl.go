package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type ACL struct {
	ID               types.String `tfsdk:"id"`
	VirtualClusterID types.String `tfsdk:"virtual_cluster_id"`
	Host             types.String `tfsdk:"host"`
	Principal        types.String `tfsdk:"principal"`
	Operation        types.String `tfsdk:"operation"`
	PermissionType   types.String `tfsdk:"permission_type"`
	ResourceType     types.String `tfsdk:"resource_type"`
	ResourceName     types.String `tfsdk:"resource_name"`
	PatternType      types.String `tfsdk:"pattern_type"`
	CreatedAt        types.String `tfsdk:"created_at"`
}
