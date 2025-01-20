package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccApplicationKeyResource(t *testing.T) {
	name := "akn_test_application_key" + nameSuffix

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResource(name),
				Check:  testAccApplicationKeyResourceCheck(name),
			},
		},
	})
}

func testAccApplicationKeyResource(name string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_application_key" "test" {
  name = "%s"
}`, name)
}

func testAccApplicationKeyResourceCheck(name string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_application_key.test", "id"),
		resource.TestCheckResourceAttr("warpstream_application_key.test", "name", name),
		resource.TestCheckResourceAttrSet("warpstream_application_key.test", "key"),
		resource.TestCheckResourceAttrSet("warpstream_application_key.test", "created_at"),
	)
}
