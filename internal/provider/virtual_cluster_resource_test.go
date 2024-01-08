package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVirtualClusterResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + `resource "warpstream_virtual_cluster" "test" {
					name = "vcn_test_acc"
				}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "id"),
					resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_id"),
					resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "created_at"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "agent_pool_name", "apn_test_acc"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "default", "false"),
				),
			},
		},
	})
}
