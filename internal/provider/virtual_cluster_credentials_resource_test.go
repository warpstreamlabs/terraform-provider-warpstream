package provider

import (
	"fmt"
	"regexp"
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
			{
				Config: testAccVirtualClusterCredentialsResource_vcField("virtual_cluster_id"),
				Check:  testAccVirtualClusterCredentialsResourceCheck(false),
			},
			{
				Config: testAccVirtualClusterCredentialsResource_vcField("virtual_cluster"),
				Check:  testAccVirtualClusterCredentialsResourceCheck(false),
			},
			{
				Config:      testAccVirtualClusterCredentialsResource_vcFieldMissing(),
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
			// Workaround: re-run the first check so the TF framework cleans up the one with the error above.
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(true),
				Check:  testAccVirtualClusterCredentialsResourceCheck(true),
			},
		},
	})
}

func testAccVirtualClusterCredentialsResource_withSuperuser(su bool) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
}

resource "warpstream_virtual_cluster_credentials" "test" {
	name            = "ccn_test_%s"
	agent_pool      = warpstream_virtual_cluster.default.agent_pool_id
	virtual_cluster_id = warpstream_virtual_cluster.default.id
	cluster_superuser = %t
  }
`, nameSuffix, nameSuffix, su)
}

func testAccVirtualClusterCredentialsResource_vcField(vcFieldName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
}

resource "warpstream_virtual_cluster_credentials" "test" {
	name            = "ccn_test_%s"
	agent_pool      = warpstream_virtual_cluster.default.agent_pool_id
	%s = warpstream_virtual_cluster.default.id
	cluster_superuser = false
  }
`, nameSuffix, nameSuffix, vcFieldName)
}

func testAccVirtualClusterCredentialsResource_vcFieldMissing() string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
}

resource "warpstream_virtual_cluster_credentials" "test" {
	name            = "ccn_test_%s"
	agent_pool      = warpstream_virtual_cluster.default.agent_pool_id
	cluster_superuser = false
  }
`, nameSuffix, nameSuffix)
}

func testAccVirtualClusterCredentialsResourceCheck(su bool) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster_credentials.test", "username"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster_credentials.test", "password"),
		// resource.TestCheckResourceAttrSet("warpstream_virtual_cluster_credentials.test", "created_at"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster_credentials.test", "name", "ccn_test_"+nameSuffix),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster_credentials.test", "cluster_superuser", fmt.Sprintf("%t", su)),
	)
}
