package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccAgentKeyDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyDataSource(),
				Check:  testAccAgentKeyDataSourceCheck(),
			},
		},
	})
}

func testAccAgentKeyDataSource() string {
	return providerConfig + `
data "warpstream_agent_keys" "test" {
}`
}

func testAccAgentKeyDataSourceCheck() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("data.warpstream_agent_keys.test", "agent_keys.#"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_agent_keys.test", "agent_keys.0.name", "akn_"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_agent_keys.test", "agent_keys.0.key", "aks_"),
	)
}
