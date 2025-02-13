package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccVirtualClusterCredentialsResourceDeletePlan(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create crednetial
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(true),
				Check:  testAccVirtualClusterCredentialsResourceCheck(true),
			},
			// Pre delete credential and try planning
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					vcs, err := client.GetVirtualClusters()
					require.NoError(t, err)

					var virtualCluster api.VirtualCluster
					for _, vc := range vcs {
						if vc.Name == fmt.Sprintf("vcn_%s", nameSuffix) {
							virtualCluster = vc
							break
						}
					}
					require.NotEmpty(t, virtualCluster.ID)

					credentials, err := client.GetCredentials(virtualCluster)
					require.NoError(t, err)

					var vcCredentialID string
					for cID, credential := range credentials {
						if credential.Name == fmt.Sprintf("ccn_test_%s", nameSuffix) {
							vcCredentialID = cID
							break
						}
					}
					require.NotEmpty(t, vcCredentialID)

					err = client.DeleteCredentials(vcCredentialID, virtualCluster)
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_virtual_cluster_credentials.test", plancheck.ResourceActionCreate),
					},
				},
			},
			// Create credential
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(true),
				Check:  testAccVirtualClusterCredentialsResourceCheck(true),
			},
			// Delete virtual cluster and try planning
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					vcs, err := client.GetVirtualClusters()
					require.NoError(t, err)

					var virtualCluster api.VirtualCluster
					for _, vc := range vcs {
						if vc.Name == fmt.Sprintf("vcn_%s", nameSuffix) {
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
						plancheck.ExpectResourceAction("warpstream_virtual_cluster.default", plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction("warpstream_virtual_cluster_credentials.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

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
