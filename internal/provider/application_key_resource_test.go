package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

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

func TestAccApplicationKeyResourceDeletePLan(t *testing.T) {
	name := "akn_test_application_key" + nameSuffix

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResource(name),
				Check:  testAccApplicationKeyResourceCheck(name),
			},
			{
				PreConfig: func() {
					client := getProviderClient(t)

					apiKeys, err := client.GetAPIKeys()
					require.NoError(t, err)

					var apiKeyID string
					for _, apiKey := range apiKeys {
						if apiKey.Name == name {
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
	name := "akn_test_application_key" + nameSuffix
	workspaces := getWorkspacesNotEmpty(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResource(name),
				// Defaults to the first i.e. the oldest workspace.
				Check: testAccApplicationKeyResourceCheckWithWorkspaceID(name, workspaces[0].ID),
			},
		},
	})
}

func TestAccApplicationKeyResourceWithWorkspaceID(t *testing.T) {
	name := "akn_test_application_key" + nameSuffix
	workspaces := getWorkspacesNotEmpty(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyResourceWithWorkspaceID(name, workspaces[0].ID),
				Check:  testAccApplicationKeyResourceCheckWithWorkspaceID(name, workspaces[0].ID),
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

func testAccApplicationKeyResourceWithWorkspaceID(name, workspaceID string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_application_key" "test" {
  name = "%s"
  workspace_id = "%s"
}`, name, workspaceID)
}

func testAccApplicationKeyResourceCheck(name string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_application_key.test", "id"),
		resource.TestCheckResourceAttr("warpstream_application_key.test", "name", name),
		resource.TestCheckResourceAttrSet("warpstream_application_key.test", "key"),
		resource.TestCheckResourceAttrSet("warpstream_application_key.test", "workspace_id"),
		resource.TestCheckResourceAttrSet("warpstream_application_key.test", "created_at"),
	)
}

func testAccApplicationKeyResourceCheckWithWorkspaceID(name, workspaceID string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccApplicationKeyResourceCheck(name),
		resource.TestCheckResourceAttr("warpstream_application_key.test", "workspace_id", workspaceID),
	)
}
