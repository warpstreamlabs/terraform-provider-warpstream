package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccVirtualClusterResource(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterResource_withPartialConfiguration(false),
				Check:  testAccVirtualClusterResourceCheck_BYOC(false, true, 1),
			},
			{
				Config: testAccVirtualClusterResource(vcNameSuffix),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccVirtualClusterResource_withConfiguration(true, false, 2, vcNameSuffix),
				Check:  testAccVirtualClusterResourceCheck_BYOC(true, false, 2),
			},
		},
	})
}

func testAccVirtualClusterResource(vcNameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
}`, vcNameSuffix)
}

func testAccVirtualClusterResource_withPartialConfiguration(acls bool) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  configuration = {
    enable_acls = %t
  }
}`, nameSuffix, acls)
}

func testAccVirtualClusterResource_withConfiguration(acls bool, autoTopic bool, numParts int64, nameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  configuration = {
    enable_acls = %t
    default_num_partitions = %d
    auto_create_topic = %t
  }
}`, nameSuffix, acls, numParts, autoTopic)
}

func testAccVirtualClusterResourceCheck_BYOC(acls bool, autoTopic bool, numParts int64) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		testAccVirtualClusterResourceCheck(acls, autoTopic, numParts, "byoc"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "agent_keys.#", "1"),
		utils.TestCheckResourceAttrStartsWith("warpstream_virtual_cluster.test", "agent_keys.0.name", "akn_virtual_cluster_test_acc_"),
		utils.TestCheckResourceAttrEndsWith("warpstream_virtual_cluster.test", "bootstrap_url", ".kafka.discoveryv2.prod-z.us-east-1.warpstream.com:9092"),
	)
}

func testAccVirtualClusterResourceCheck(acls bool, autoTopic bool, numParts int64, vcType string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "created_at"),
		// Note: agent_pool_name is now equal to "apn_test_acc_"+nameSuffix + randomSuffix
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_name"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "default", "false"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "type", vcType),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_acls", fmt.Sprintf("%t", acls)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.auto_create_topic", fmt.Sprintf("%t", autoTopic)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_num_partitions", fmt.Sprintf("%d", numParts)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_retention_millis", fmt.Sprintf("%d", 86400000)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "cloud.provider", "aws"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "cloud.region", "us-east-1"),
	)
}

func TestAccVirtualClusterImport(t *testing.T) {
	os.Setenv("WARPSTREAM_API_KEY", "aks_51a3819b5f31d4bf9e313da2e2b39c412ab23de7771fec166cdd611d2910f72e")

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterResource(vcNameSuffix),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "warpstream_virtual_cluster.test",
			},
		},
		IsUnitTest: true,
	})
}
