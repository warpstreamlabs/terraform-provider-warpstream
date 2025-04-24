package shared

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

var VirtualClusterWorkspaceIDSchema = schema.StringAttribute{
	Description: "Workspace ID. " +
		"ID of the workspace to which the virtual cluster belongs. " +
		"Assigned based on the workspace of the application key used to authenticate the WarpStream provider. " +
		"Cannot be changed after creation.",
	Computed: true,
	PlanModifiers: []planmodifier.String{
		stringplanmodifier.UseStateForUnknown(),
	},
}
