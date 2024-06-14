package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccVirtualClusterDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterDataSource_default(),
				Check:  testAccVCDataSourceCheckBYOC("default"),
			},
			{
				Config: testAccVirtualClusterDataSource_serverless(),
				Check:  testAccVCDataSourceCheckServerless(),
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

func testAccVirtualClusterDataSource_serverless() string {
	return providerConfig + `
data "warpstream_virtual_cluster" "test" {
  name = "vcn_tivo_serverless"
}`
}

func testAccVCDataSourceCheckBYOC(name string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "created_at"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_virtual_cluster.test", "agent_pool_name", "apn_"+name),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "type", "byoc"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.provider", "aws"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.region", "us-east-1"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "agent_keys.#", "1"),
		resource.TestCheckResourceAttr(
			"data.warpstream_virtual_cluster.test", "agent_keys.0.name", "akn_virtual_cluster_default_7695dba1efaa",
		),
	)
}

func testAccVCDataSourceCheckServerless() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "type", "serverless"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.provider", "aws"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.region", "us-east-1"),
		resource.TestCheckNoResourceAttr("data.warpstream_virtual_cluster.test", "agent_keys"),
	)
}
