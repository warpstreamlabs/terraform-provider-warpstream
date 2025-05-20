package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAccountKeyWorkspaceResource(t *testing.T) {
	workspaceNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceResource(workspaceNameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_workspace.test", "id"),
					resource.TestCheckResourceAttr("warpstream_workspace.test", "name", fmt.Sprintf("test_acc_%s", workspaceNameSuffix)),
					resource.TestCheckResourceAttrSet("warpstream_workspace.test", "created_at"),
				),
			},
			{
				Config: testAccWorkspaceResource(workspaceNameSuffix + "_renamed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_workspace.test", "name", fmt.Sprintf("test_acc_%s_renamed", workspaceNameSuffix)),
				),
			},
		},
	})
}

func testAccWorkspaceResource(nameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_workspace" "test" {
  name = "test_acc_%s"
}`, nameSuffix)
}
