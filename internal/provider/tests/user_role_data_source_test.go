package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccAccountKeyUserRoleDataSource(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	readonlyRoleName := "readonly_test" + nameSuffix
	readonlyRoleID, err := client.CreateUserRole(readonlyRoleName, []api.AccessGrant{{ManagedGrantKey: "read_only", WorkspaceID: "*", ResourceID: "*"}})
	require.NoError(t, err)

	defer func() {
		err = client.DeleteUserRole(readonlyRoleID)
		require.NoError(t, err)
	}()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserRoleDataSourceWithID(readonlyRoleID),
				Check:  testAccUserRoleDataSourceCheck(readonlyRoleID, readonlyRoleName),
			},
			{
				Config: testAccUserRoleDataSourceWithName(readonlyRoleName),
				Check:  testAccUserRoleDataSourceCheck(readonlyRoleID, readonlyRoleName),
			},
		},
	})
}

func testAccUserRoleDataSourceWithID(id string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_user_role" "read_only_role" {
  id = "%s"
}`, id)
}

func testAccUserRoleDataSourceWithName(name string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_user_role" "read_only_role" {
  name = "%s"
}`, name)
}

func testAccUserRoleDataSourceCheck(readonlyRoleID, readonlyRoleName string) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.warpstream_user_role.read_only_role", "id", readonlyRoleID),
		resource.TestCheckResourceAttr("data.warpstream_user_role.read_only_role", "name", readonlyRoleName),
		resource.TestCheckResourceAttr("data.warpstream_user_role.read_only_role", "access_grants.#", "1"),
		resource.TestCheckResourceAttr("data.warpstream_user_role.read_only_role", "access_grants.0.workspace_id", "*"),
		resource.TestCheckResourceAttr("data.warpstream_user_role.read_only_role", "access_grants.0.grant_type", "read_only"),
	)
}
