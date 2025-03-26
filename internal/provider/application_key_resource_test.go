package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccApplicationKeyResourceDeletePLan(t *testing.T) {
	resourceName := "test"
	keyName := "akn_test_application_key" + nameSuffix

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResource(resourceName, keyName),
				Check:  testAccApplicationKeyResourceCheck(resourceName, keyName),
			},
			{
				PreConfig: func() {
					client := getProviderClient(t)

					apiKeys, err := client.GetAPIKeys()
					require.NoError(t, err)

					var apiKeyID string
					for _, apiKey := range apiKeys {
						if apiKey.Name == keyName {
							apiKeyID = apiKey.ID
							break
						}
					}
					require.NotEmpty(t, apiKeyID)

					err = client.DeleteAPIKey(apiKeyID)
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_application_key.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccApplicationKeyResource(t *testing.T) {
	resourceName := "test"
	keyName := "akn_test_application_key" + nameSuffix
	workspaces := getWorkspacesNotEmpty(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResource(resourceName, keyName),
				// Defaults to the first i.e. the oldest workspace.
				Check: testAccApplicationKeyResourceCheckWithWorkspaceID(resourceName, keyName, workspaces[0].ID),
			},
		},
	})
}

func TestAccApplicationKeyResourceWithWorkspaceID(t *testing.T) {
	keyName := "akn_test_application_key" + nameSuffix
	workspaces := getWorkspacesNotEmpty(t)
	resourceName := "test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResourceWithWorkspaceID(resourceName, keyName, workspaces[0].ID),
				Check:  testAccApplicationKeyResourceCheckWithWorkspaceID(resourceName, keyName, workspaces[0].ID),
			},
			{
				Config:      testAccApplicationKeyResourceWithWorkspaceID(resourceName, keyName, "wi_not_exist"),
				ExpectError: regexp.MustCompile("workspace not found."),
			},
		},
	})
}

func TestAccAccountKeyApplicationKeyResourceWithWorkspaceID(t *testing.T) {
	resourceName1, resourceName2 := "test1", "test2"
	keyName1, keyName2 := "akn_test_application_key"+nameSuffix+"_1", "akn_test_application_key"+nameSuffix+"_2"
	client := getProviderClient(t)
	workspaces, err := client.GetWorkspaces()
	require.NoError(t, err)
	require.Greater(t, len(workspaces), 1, "Are you running this test with an account key?") // Get at least two workspaces.

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Can manage application keys in different workspaces.
				Config: testAccApplicationKeyResourceMultipleWorkspaces(resourceName1, resourceName2, keyName1, keyName2, workspaces[0].ID, workspaces[1].ID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccApplicationKeyResourceCheckWithWorkspaceID(resourceName1, keyName1, workspaces[0].ID),
					testAccApplicationKeyResourceCheckWithWorkspaceID(resourceName2, keyName2, workspaces[1].ID),
				),
			},
			{
				// Can't duplicate key names.
				Config:      testAccApplicationKeyResourceMultipleWorkspaces(resourceName1, resourceName2, keyName1, keyName1, workspaces[0].ID, workspaces[0].ID),
				ExpectError: regexp.MustCompile("duplicate_api_key_name"),
			},
		},
	})
}

func testAccApplicationKeyResource(resourceName, keyName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_application_key" "%s" {
  name = "%s"
}`, resourceName, keyName)
}

func testAccApplicationKeyResourceWithWorkspaceID(resourceName, keyName, workspaceID string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_application_key" "%s" {
  name = "%s"
  workspace_id = "%s"
}`, resourceName, keyName, workspaceID)
}

func testAccApplicationKeyResourceMultipleWorkspaces(resourceName1, resourceName2, keyName1, keyName2, workspaceID1, workspaceID2 string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_application_key" "%s" {
  name = "%s"
  workspace_id = "%s"
}

resource "warpstream_application_key" "%s" {
  name = "%s"
  workspace_id = "%s"
}`, resourceName1, keyName1, workspaceID1, resourceName2, keyName2, workspaceID2)
}

func testAccApplicationKeyResourceCheck(resourceName, keyName string) resource.TestCheckFunc {
	resourcePath := getApplicationKeyResourcePath(resourceName)
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(resourcePath, "id"),
		resource.TestCheckResourceAttr(resourcePath, "name", keyName),
		resource.TestCheckResourceAttrSet(resourcePath, "key"),
		resource.TestCheckResourceAttrSet(resourcePath, "workspace_id"),
		resource.TestCheckResourceAttrSet(resourcePath, "created_at"),
	)
}

func testAccApplicationKeyResourceCheckWithWorkspaceID(resourceName, keyName, workspaceID string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccApplicationKeyResourceCheck(resourceName, keyName),
		resource.TestCheckResourceAttr(getApplicationKeyResourcePath(resourceName), "workspace_id", workspaceID),
	)
}

func getProviderClient(t *testing.T) *api.Client {
	token, host := os.Getenv("WARPSTREAM_API_KEY"), os.Getenv("WARPSTREAM_API_URL")
	client, err := api.NewClient(host, &token)
	require.NoError(t, err)
	return client
}

func getWorkspacesNotEmpty(t *testing.T) []api.Workspace {
	client := getProviderClient(t)
	workspaces, err := client.GetWorkspaces()
	require.NoError(t, err)
	require.NotEmpty(t, workspaces)
	return workspaces
}

func getApplicationKeyResourcePath(resourceName string) string {
	return fmt.Sprintf("warpstream_application_key.%s", resourceName)
}
