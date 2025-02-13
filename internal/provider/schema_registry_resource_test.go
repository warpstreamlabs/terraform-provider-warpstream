package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func testSchemaRegistryResource(nameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_schema_registry" "test" {
  name = "vcn_sr_test_%s"
}`, nameSuffix)
}

func TestAccSchemaRegistryResourceDeletePlan(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resourceName := "warpstream_schema_registry.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testSchemaRegistryResource(vcNameSuffix),
				Check:  testCheckSchemaRegistry(resourceName),
			},
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					vcs, err := client.GetVirtualClusters()
					require.NoError(t, err)

					var virtualCluster api.VirtualCluster
					for _, vc := range vcs {
						if vc.Name == fmt.Sprintf("vcn_sr_test_%s", vcNameSuffix) {
							virtualCluster = vc
							break
						}
					}
					require.NotEmpty(t, virtualCluster.ID)

					err = client.DeleteVirtualCluster(virtualCluster.ID, virtualCluster.Name)
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_schema_registry.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccSchemaRegistryResource(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resourceName := "warpstream_schema_registry.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testSchemaRegistryResource(vcNameSuffix),
				Check:  testCheckSchemaRegistry(resourceName),
			},
		},
	})
}

func testCheckSchemaRegistry(resourceName string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(resourceName, "id"),
		resource.TestCheckResourceAttrSet(resourceName, "created_at"),
		resource.TestCheckResourceAttr(resourceName, "cloud.provider", "aws"),
		resource.TestCheckResourceAttr(resourceName, "cloud.region", "us-east-1"),
		utils.TestCheckResourceAttrStartsWith(resourceName, "id", "vci_sr_"),
	)
}

func TestAccSchemaRegistryImport(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testSchemaRegistryResource(vcNameSuffix),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "warpstream_schema_registry.test",
			},
		},
	})
}
