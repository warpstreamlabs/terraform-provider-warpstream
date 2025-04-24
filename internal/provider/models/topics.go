package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type TopicConfig struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

type Topic struct {
	ID               types.String  `tfsdk:"id"`
	VirtualClusterID types.String  `tfsdk:"virtual_cluster_id"`
	TopicName        types.String  `tfsdk:"topic_name"`
	PartitionCount   types.Int64   `tfsdk:"partition_count"`
	Config           []TopicConfig `tfsdk:"config"`
}
