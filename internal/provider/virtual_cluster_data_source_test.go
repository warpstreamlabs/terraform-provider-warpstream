package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVirtualClusterDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterDataSource_default(),
				Check:  testAccVirtualClusterDataSourceCheck("default"),
			},
		},
	})
}

func testAccVirtualClusterDataSource_default() string {
	return providerConfig + `
data "warpstream_virtual_cluster" "test" {
  default = true
}`
}

func testAccVirtualClusterDataSourceCheck(name string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "created_at"),
		resource.TestCheckResourceAttrWith(
			"data.warpstream_virtual_cluster.test",
			"agent_pool_name",
			func(value string) error {
				if !strings.HasPrefix(value, "apn_"+name) {
					return fmt.Errorf("expected agent_pool_name to start with 'apn_%s', got: %s", name, value)
				}
				return nil
			},
		),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "type", "byoc"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.provider", "aws"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.region", "us-east-1"),
	)
}
