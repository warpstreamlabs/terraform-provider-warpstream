package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVirtualClustersDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: providerConfig + `data "warpstream_virtual_clusters" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify number of virtual clusters returned
					resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.#", "1"),
					// Verify the first (default) virtual clusters to ensure all attributes are set
					resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.0.name", "vcn_default"),
					resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.0.agent_pool_name", "apn_default"),
				),
			},
		},
	})
}
