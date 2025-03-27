package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccAccountKeyWorkspaceDataSource(t *testing.T) {
	workspaceName := "test_workspace_" + nameSuffix
	client, err := api.NewClientDefault()
	require.NoError(t, err)
	workspaceID, err := client.CreateWorkspace(workspaceName)
	require.NoError(t, err)

	defaultAppKeyName := "akn_" + workspaceName + "_default_application_key" // Generated in the WarpStream backend.
	otherAppKeyName := "akn_test_workspace_application_key_" + nameSuffix

	_, err = client.CreateApplicationKey(otherAppKeyName, workspaceID)
	require.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceDataSource(workspaceID),
				Check:  testAccWorkspaceDataSourceCheck(workspaceID, workspaceName, defaultAppKeyName, otherAppKeyName),
			},
		},
	})
}

func testAccWorkspaceDataSource(id string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_workspace" "test" {
	id = "%s"
}`, id)
}

func testAccWorkspaceDataSourceCheck(workspaceID, workspaceName, appKey1Name, appKey2Name string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.warpstream_workspace.test", "id", workspaceID),
		resource.TestCheckResourceAttr("data.warpstream_workspace.test", "name", workspaceName),
		resource.TestCheckResourceAttrSet("data.warpstream_workspace.test", "application_keys.#"),
		resource.TestCheckResourceAttr("data.warpstream_workspace.test", "application_keys.0.name", appKey1Name),
		resource.TestCheckResourceAttr("data.warpstream_workspace.test", "application_keys.1.name", appKey2Name),
	)
}
