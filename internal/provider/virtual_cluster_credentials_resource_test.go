package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccVirtualClusterCredentialsResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(true),
				Check:  testAccVirtualClusterCredentialsResourceCheck(true),
			},
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_virtual_cluster_credentials.test", plancheck.ResourceActionReplace),
						plancheck.ExpectSensitiveValue("warpstream_virtual_cluster_credentials.test", tfjsonpath.New("password")),
					},
				},
			},
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(false),
				Check:  testAccVirtualClusterCredentialsResourceCheck(false),
			},
		},
	})
}

func testAccVirtualClusterCredentialsResource_withSuperuser(su bool) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_virtual_cluster" "default" {
	default = true
}

resource "warpstream_virtual_cluster_credentials" "test" {
	name            = "ccn_test_%s"
	agent_pool      = data.warpstream_virtual_cluster.default.agent_pool_id
	virtual_cluster = data.warpstream_virtual_cluster.default.id
	cluster_superuser = %t
  }
`, nameSuffix, su)
}

func testAccVirtualClusterCredentialsResourceCheck(su bool) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster_credentials.test", "username"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster_credentials.test", "password"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster_credentials.test", "created_at"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster_credentials.test", "name", "ccn_test_"+nameSuffix),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster_credentials.test", "cluster_superuser", fmt.Sprintf("%t", su)),
	)
}
