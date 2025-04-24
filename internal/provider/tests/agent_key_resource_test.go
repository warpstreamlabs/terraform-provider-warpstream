package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccAgentKeyResourceDeletePlan(t *testing.T) {
	name := "akn_test_agent_key" + nameSuffix
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResource(name, vcID),
				Check:  testAccAgentKeyResourceCheck(name, vcID),
			},
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

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
						plancheck.ExpectResourceAction("warpstream_agent_key.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccAgentKeyResource(t *testing.T) {
	name := "akn_test_agent_key" + nameSuffix
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
