package tests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccVirtualClusterCredentialsResourceDeletePlan(t *testing.T) {
	vcName := utils.CreateTestKafkaVcNameWithNamespace("cred")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create credential
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(true, vcName),
				Check:  testAccVirtualClusterCredentialsResourceCheck(true),
			},
			// Pre delete credential and try planning
			{
				PreConfig: func() {
					client, err := api.NewClientDefault()
					require.NoError(t, err)

					virtualCluster, err := client.FindVirtualCluster(vcName)
					require.NoError(t, err)

					credentials, err := client.GetCredentials(*virtualCluster)
					require.NoError(t, err)

					var vcCredentialID string
					for cID, credential := range credentials {
						if credential.Name == fmt.Sprintf("ccn_test_%s", nameSuffix) {
							vcCredentialID = cID
							break
						}
					}
					require.NotEmpty(t, vcCredentialID)

					err = client.DeleteCredentials(vcCredentialID, *virtualCluster)
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
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(true, vcName),
				Check:  testAccVirtualClusterCredentialsResourceCheck(true),
			},
			// Delete virtual cluster and try planning
			{
				PreConfig: func() {
					client, err := api.NewClientDefault()
					require.NoError(t, err)

					virtualCluster, err := client.FindVirtualCluster(vcName)
					require.NoError(t, err)

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
	vcName := utils.CreateTestKafkaVcNameWithNamespace("cred")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(true, vcName),
				Check:  testAccVirtualClusterCredentialsResourceCheck(true),
			},
			{
				ResourceName: "warpstream_virtual_cluster_credentials.test",
				ImportState:  true,
				// Normally we would set this to true, however when importing the password becomes
				// nil. So we cannot actually verify the state.
				// ImportStateVerify: true,
			},
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(false, vcName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_virtual_cluster_credentials.test", plancheck.ResourceActionReplace),
						plancheck.ExpectSensitiveValue("warpstream_virtual_cluster_credentials.test", tfjsonpath.New("password")),
					},
				},
			},
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(false, vcName),
				Check:  testAccVirtualClusterCredentialsResourceCheck(false),
			},
			{
				Config: testAccVirtualClusterCredentialsResource_vcField("virtual_cluster_id", vcName),
				Check:  testAccVirtualClusterCredentialsResourceCheck(false),
			},
			{
				Config: testAccVirtualClusterCredentialsResource_vcField("virtual_cluster", vcName),
				Check:  testAccVirtualClusterCredentialsResourceCheck(false),
			},
			{
				Config:      testAccVirtualClusterCredentialsResource_vcFieldMissing(vcName),
				ExpectError: regexp.MustCompile("Invalid Attribute Combination"),
			},
			// Workaround: re-run the first check so the TF framework cleans up the one with the error above.
			{
				Config: testAccVirtualClusterCredentialsResource_withSuperuser(true, vcName),
				Check:  testAccVirtualClusterCredentialsResourceCheck(true),
			},
		},
	})
}

func testAccVirtualClusterCredentialsResource_withSuperuser(su bool, vcName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "%s"
    tier = "dev"
}

resource "warpstream_virtual_cluster_credentials" "test" {
	name            = "ccn_test_%s"
	virtual_cluster_id = warpstream_virtual_cluster.default.id
	cluster_superuser = %t
  }
`, vcName, nameSuffix, su)
}

func testAccVirtualClusterCredentialsResource_vcField(vcFieldName string, vcName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "%s"
    tier = "dev"
}

resource "warpstream_virtual_cluster_credentials" "test" {
	name            = "ccn_test_%s"
	%s = warpstream_virtual_cluster.default.id
	cluster_superuser = false
  }
`, vcName, nameSuffix, vcFieldName)
}

func testAccVirtualClusterCredentialsResource_vcFieldMissing(vcName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "%s"
    tier = "dev"
}

resource "warpstream_virtual_cluster_credentials" "test" {
	name            = "ccn_test_%s"
	cluster_superuser = false
  }
`, vcName, nameSuffix)
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
