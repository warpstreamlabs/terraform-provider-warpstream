package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAgentKeyResource(t *testing.T) {
	name := "akn_test_agent_key"
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResource(name, vcID),
				Check:  testAccAgentKeyResourceCheck(name, vcID),
			},
		},
	})
}

func testAccAgentKeyResource(name, vcID string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_agent_key" "test" {
  name = "%s"
  virtual_cluster_id = "%s"
}`, name, vcID)
}

func testAccAgentKeyResourceCheck(name, vcID string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "id"),
		resource.TestCheckResourceAttr("warpstream_agent_key.test", "name", name),
		resource.TestCheckResourceAttr("warpstream_agent_key.test", "virtual_cluster_id", vcID),
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "key"),
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "created_at"),
	)
}
