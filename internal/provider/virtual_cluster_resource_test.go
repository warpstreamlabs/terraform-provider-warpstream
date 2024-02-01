package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccVirtualClusterResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterResource_withConfiguration(false),
				Check:  testAccVirtualClusterResourceCheck(false),
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
				Config: testAccVirtualClusterResource_withConfiguration(true),
				Check:  testAccVirtualClusterResourceCheck(true),
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

func testAccVirtualClusterResource_withConfiguration(acls bool) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
	name = "vcn_test_acc_%s"
	configuration = {
		enable_acls = %t
	}
}`, nameSuffix, acls)
}

func testAccVirtualClusterResourceCheck(acls bool) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "created_at"),
		// Note: agent_pool_name is now equal to "apn_test_acc_"+nameSuffix + randomSuffix
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_name"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "default", "false"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_acls", fmt.Sprintf("%t", acls)),
	)
}
