package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func testTableFlowResource(nameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_tableflow_cluster" "test" {
  name = "vcn_dl_test_%s"
  tier = "dev"
}`, nameSuffix)
}

func TestAccTableFlowResourceDeletePlan(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resourceName := "warpstream_tableflow.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTableFlowResource(vcNameSuffix),
				Check:  testCheckTableFlow(resourceName),
			},
			{
				PreConfig: func() {
					client, err := api.NewClientDefault()
					require.NoError(t, err)

					virtualCluster, err := client.FindVirtualCluster(fmt.Sprintf("vcn_dl_test_%s", vcNameSuffix))
					require.NoError(t, err)

					err = client.DeleteVirtualCluster(virtualCluster.ID, virtualCluster.Name)
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_tableflow.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccTableFlowResource(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resourceName := "warpstream_tableflow.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTableFlowResource(vcNameSuffix),
				Check:  testCheckTableFlow(resourceName),
			},
		},
	})
}

func testCheckTableFlow(resourceName string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(resourceName, "id"),
		resource.TestCheckResourceAttrSet(resourceName, "created_at"),
		resource.TestCheckResourceAttrSet(resourceName, "tier"),
		resource.TestCheckResourceAttr(resourceName, "cloud.provider", "aws"),
		resource.TestCheckResourceAttr(resourceName, "cloud.region", "us-east-1"),
		utils.TestCheckResourceAttrStartsWith(resourceName, "id", "vci_dl_"),
	)
}

func TestAccTableFlowImport(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTableFlowResource(vcNameSuffix),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "warpstream_tableflow.test",
			},
		},
	})
}
