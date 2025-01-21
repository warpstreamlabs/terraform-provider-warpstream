package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccApplicationKeyDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyDataSource(),
				Check:  testAccApplicationKeyDataSourceCheck(),
			},
		},
	})
}

func testAccApplicationKeyDataSource() string {
	return providerConfig + `
data "warpstream_application_keys" "test" {
}`
}

func testAccApplicationKeyDataSourceCheck() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("data.warpstream_application_keys.test", "application_keys.#"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_application_keys.test", "application_keys.0.name", "akn_"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_application_keys.test", "application_keys.0.key", "aks_"),
	)
}
