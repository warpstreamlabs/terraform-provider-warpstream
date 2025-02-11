package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

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
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					apiKeys, err := client.GetAPIKeys()
					require.NoError(t, err)

					var apiKeyID *string
					for _, apiKey := range apiKeys {
						if apiKey.Name == name {
							apiKeyID = &apiKey.ID
							break
						}
					}
					require.NotNil(t, apiKeyID)

					err = client.DeleteAPIKey(*apiKeyID)
					require.NoError(t, err)
				},
				Config:             testAccApplicationKeyResource(name),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

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
