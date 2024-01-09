package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVirtualClusterResource(t *testing.T) {
	// We add a random suffix at the end of the virtual cluster name
	// in order to prevent name collision when acceptance tests run
	// in parallel for different terraform version.
	suffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccVirtualClusterResourceConfig(suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "id"),
					resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_id"),
					resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "created_at"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "agent_pool_name", "apn_test_acc_"+suffix),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "default", "false"),
				),
			},
		},
	})
}

func testAccVirtualClusterResourceConfig(suffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
	name = "vcn_test_acc_%s"
}`, suffix)
}
