package tests

import (
	"fmt"
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
					client, err := api.NewClientDefault()
					require.NoError(t, err)

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
	workspace := getNonEmptyWorkspace(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResource(resourceName, keyName),
				// Defaults to the authenticated key's workspace.
				Check: testAccApplicationKeyResourceCheckWithWorkspaceID(resourceName, keyName, workspace.ID),
			},
		},
	})
}

func TestAccApplicationKeyResourceWithWorkspaceID(t *testing.T) {
	keyName := "akn_test_application_key" + nameSuffix
	workspace := getNonEmptyWorkspace(t)
	resourceName := "test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResourceWithWorkspaceID(resourceName, keyName, workspace.ID),
				Check:  testAccApplicationKeyResourceCheckWithWorkspaceID(resourceName, keyName, workspace.ID),
			},
			{
				Config:      testAccApplicationKeyResourceWithWorkspaceID(resourceName, keyName, "wi_not_exist"),
				ExpectError: regexp.MustCompile("workspace not found."),
			},
		},
	})
}

func TestAccAccountKeyApplicationKeyResourceDefaultsToOldestWorkspace(t *testing.T) {
	resourceName := "test"
	keyName := "akn_test_application_key_oldest_default" + nameSuffix
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	workspaces, err := client.GetWorkspaces()
	require.NoError(t, err)
	require.NotEmpty(t, workspaces)
	oldestID := workspaces[0].ID

	newerID1, err := client.CreateWorkspace("test_acc_newer_ws_1_" + nameSuffix)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.DeleteWorkspace(newerID1))
	})
	newerID2, err := client.CreateWorkspace("test_acc_newer_ws_2_" + nameSuffix)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.DeleteWorkspace(newerID2))
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResource(resourceName, keyName),
				// Omitting workspace_id defaults to the oldest workspace, not a newer one.
				Check: testAccApplicationKeyResourceCheckWithWorkspaceID(resourceName, keyName, oldestID),
			},
		},
	})
}

func TestAccAccountKeyApplicationKeyResourceWithWorkspaceID(t *testing.T) {
	resourceName1, resourceName2 := "test1", "test2"
	keyName1, keyName2 := "akn_test_application_key"+nameSuffix+"_1", "akn_test_application_key"+nameSuffix+"_2"
	workspaces := getWorkspacesAtLeastTwo(t)

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

func TestAccApplicationKeyResourceReadOnly(t *testing.T) {
	resourceName := "test"
	keyName := "akn_test_application_key_readonly" + nameSuffix
	workspace := getNonEmptyWorkspace(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResourceWithReadOnly(resourceName, keyName, true),
				Check:  testAccApplicationKeyResourceCheckWithReadOnly(resourceName, keyName, workspace.ID, "true"),
			},
		},
	})
}

func TestAccApplicationKeyResourceNotReadOnly(t *testing.T) {
	resourceName := "test"
	keyName := "akn_test_application_key_not_readonly" + nameSuffix
	workspace := getNonEmptyWorkspace(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResourceWithReadOnly(resourceName, keyName, false),
				Check:  testAccApplicationKeyResourceCheckWithReadOnly(resourceName, keyName, workspace.ID, "false"),
			},
		},
	})
}

func TestAccApplicationKeyResourceReadOnlyRequiresReplace(t *testing.T) {
	resourceName := "test"
	keyName := "akn_test_application_key_replace" + nameSuffix
	workspace := getNonEmptyWorkspace(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResourceWithReadOnly(resourceName, keyName, false),
				Check:  testAccApplicationKeyResourceCheckWithReadOnly(resourceName, keyName, workspace.ID, "false"),
			},
			{
				Config: testAccApplicationKeyResourceWithReadOnly(resourceName, keyName, true),
				Check:  testAccApplicationKeyResourceCheckWithReadOnly(resourceName, keyName, workspace.ID, "true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_application_key.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
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

func testAccApplicationKeyResourceWithReadOnly(resourceName, keyName string, readOnly bool) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_application_key" "%s" {
  name      = "%s"
  read_only = %t
}`, resourceName, keyName, readOnly)
}

func testAccApplicationKeyResourceCheckWithReadOnly(resourceName, keyName, workspaceID, readOnly string) resource.TestCheckFunc {
	resourcePath := getApplicationKeyResourcePath(resourceName)
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(resourcePath, "id"),
		resource.TestCheckResourceAttr(resourcePath, "name", keyName),
		resource.TestCheckResourceAttrSet(resourcePath, "key"),
		resource.TestCheckResourceAttr(resourcePath, "workspace_id", workspaceID),
		resource.TestCheckResourceAttrSet(resourcePath, "created_at"),
		resource.TestCheckResourceAttr(resourcePath, "read_only", readOnly),
	)
}

func getWorkspacesAtLeastTwo(t *testing.T) []api.Workspace {
	t.Helper()
	client, err := api.NewClientDefault()
	require.NoError(t, err)
	workspaces, err := client.GetWorkspaces()
	require.NoError(t, err)
	require.Greater(t, len(workspaces), 1, "Are you running this test with an account key?")
	return workspaces
}

// getNonEmptyWorkspace returns a workspace visible to the authenticated API key.
func getNonEmptyWorkspace(t *testing.T) api.Workspace {
	client, err := api.NewClientDefault()
	require.NoError(t, err)
	// /list_workspaces requires an account key and most tests are run with an application key.
	// Instead, list API keys and grab the first key's workspace ID so that this can be called from any test.
	keys, err := client.GetAPIKeys()
	require.NoError(t, err)
	require.NotEmpty(t, keys)

	wsID := keys[0].AccessGrants.ReadWorkspaceIDSafe()
	require.NotEmpty(t, wsID)

	return api.Workspace{ID: wsID}
}

func getApplicationKeyResourcePath(resourceName string) string {
	return fmt.Sprintf("warpstream_application_key.%s", resourceName)
}
