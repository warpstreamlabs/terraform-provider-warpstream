package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAccountKeyUserRoleResource(t *testing.T) {
	randSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserRoleResource(randSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "name", fmt.Sprintf("test_acc_user_role_%s", randSuffix)),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "created_at"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "access_grants.#", "1"),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "access_grants.0.workspace_id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "access_grants.0.grant_type", "admin"),
				),
			},
		},
	})
}

func testAccUserRoleResource(randSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_workspace" "test" {
  name = "test_acc_workspace_%s"
}

resource "warpstream_user_role" "test" {
  name = "test_acc_user_role_%s"
  access_grants = [
    {
      workspace_id = warpstream_workspace.test.id
      grant_type   = "admin"
    },
  ]
}`, randSuffix, randSuffix)
}
