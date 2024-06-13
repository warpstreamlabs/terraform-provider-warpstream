package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccVirtualClusterResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterResource_withPartialConfiguration(false),
				Check:  testAccVirtualClusterResourceCheck(false, true, 1),
			},
			{
				Config: testAccVirtualClusterResource(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccVirtualClusterResource_withConfiguration(true, false, 2),
				Check:  testAccVirtualClusterResourceCheck(true, false, 2),
			},
		},
	})
}

func testAccVirtualClusterResource() string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
}`, nameSuffix)
}

func testAccVirtualClusterResource_withType(t string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  type = "%s"
}`, nameSuffix, t)
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

func testAccVirtualClusterResource_withConfiguration(acls bool, autoTopic bool, numParts int64) string {
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

func testAccVirtualClusterResourceCheck(acls bool, autoTopic bool, numParts int64) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "created_at"),
		// Note: agent_pool_name is now equal to "apn_test_acc_"+nameSuffix + randomSuffix
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_name"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "default", "false"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "type", "byoc"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_acls", fmt.Sprintf("%t", acls)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.auto_create_topic", fmt.Sprintf("%t", autoTopic)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_num_partitions", fmt.Sprintf("%d", numParts)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_retention_millis", fmt.Sprintf("%d", 86400000)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "agent_keys.#", "1"),
		resource.TestCheckResourceAttrWith(
			"warpstream_virtual_cluster.test",
			"agent_keys.0.name",
			func(value string) error {
				if !strings.HasPrefix(value, "akn_virtual_cluster_test_acc_") {
					return fmt.Errorf(
						"expected agent_pool_name to start with 'akn_virtual_cluster_test_acc_', got: %s", value,
					)
				}
				return nil
			},
		),
	)
}
