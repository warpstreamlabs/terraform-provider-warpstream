package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccVirtualClusterDataSource(t *testing.T) {
	os.Setenv("WARPSTREAM_API_KEY", "aks_51a3819b5f31d4bf9e313da2e2b39c412ab23de7771fec166cdd611d2910f72e")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterDataSource_default(),
				Check:  testAccVCDataSourceCheck_byoc("default"),
			},
		},
		IsUnitTest: true,
	})
}

func testAccVirtualClusterDataSource_default() string {
	return providerConfig + `
data "warpstream_virtual_cluster" "test" {
  default = true
}`
}

func testAccVCDataSourceCheck_byoc(name string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "type", "byoc"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "agent_keys.#", "1"),
		utils.TestCheckResourceAttrStartsWith(
			"data.warpstream_virtual_cluster.test", "agent_keys.0.virtual_cluster_id", "vci_",
		),
		utils.TestCheckResourceAttrStartsWith(
			"data.warpstream_virtual_cluster.test", "agent_keys.0.name", "akn_virtual_cluster_default_",
		),
		utils.TestCheckResourceAttrEndsWith(
			"data.warpstream_virtual_cluster.test",
			"bootstrap_url",
			".kafka.discoveryv2.prod-z.us-east-1.warpstream.com:9092",
		),
		testAccVCDataSourceCheck(name),
	)
}

func testAccVCDataSourceCheck(name string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "agent_pool_id"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_virtual_cluster.test", "agent_pool_name", "apn_"+name),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "created_at"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.provider", "aws"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.region", "us-east-1"),
	)
}
