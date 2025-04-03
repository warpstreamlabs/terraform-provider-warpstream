package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

type topicConfig struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

type topicModel struct {
	ID                        types.String  `tfsdk:"id"`
	VirtualClusterID          types.String  `tfsdk:"virtual_cluster_id"`
	TopicName                 types.String  `tfsdk:"topic_name"`
	PartitionCount            types.Int64   `tfsdk:"partition_count"`
	DeletionProtectionEnabled types.Bool    `tfsdk:"enable_deletion_protection"`
	Config                    []topicConfig `tfsdk:"config"`
}
