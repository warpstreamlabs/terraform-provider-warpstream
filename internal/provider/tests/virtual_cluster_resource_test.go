package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccVirtualClusterResourceDeletePlan(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterResource_withPartialConfiguration(false, vcNameSuffix),
				Check:  testAccVirtualClusterResourceCheck(false, true, 1, "byoc", false, false),
			},
			{
				PreConfig: func() {
					client, err := api.NewClientDefault()
					require.NoError(t, err)

					virtualCluster, err := client.FindVirtualCluster(fmt.Sprintf("vcn_test_acc_%s", vcNameSuffix))
					require.NoError(t, err)

					err = client.DeleteVirtualCluster(virtualCluster.ID, virtualCluster.Name)
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_virtual_cluster.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccVirtualClusterResource(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	var clusterID string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterResource_withPartialConfiguration(false, vcNameSuffix),
				Check:  testAccVirtualClusterResourceCheck(false, true, 1, "byoc", false, false),
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
				Check:  testAccVirtualClusterResourceCheck(true, false, 2, "byoc", true, true),
			},
			{
				Config: testAccVirtualClusterResource_removeDeletionProtection(vcNameSuffix),
				Check:  testNoDeletionProtection(),
			},
			{
				Config: testAccVirtualClusterResource_removeDeletionProtection(vcNameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					testNoDeletionProtection(),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["warpstream_virtual_cluster.test"]
						if !ok {
							return fmt.Errorf("not found: warpstream_virtual_cluster.test")
						}
						// Hold onto the cluster ID to assert that it's the same one being renamed in the next step.
						clusterID = rs.Primary.ID
						return nil
					},
				),
			},
			{
				Config: testAccVirtualClusterResource_withRenamedCluster(vcNameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccVirtualClusterResourceCheck(false, true, 1, "byoc", false, false),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "name", fmt.Sprintf("vcn_test_acc_renamed_%s", vcNameSuffix)),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["warpstream_virtual_cluster.test"]
						if !ok {
							return fmt.Errorf("not found: warpstream_virtual_cluster.test")
						}
						if rs.Primary.ID != clusterID {
							return fmt.Errorf("expected cluster ID %s, got %s", clusterID, rs.Primary.ID)
						}
						return nil
					},
				),
			},
		},
	})
}

func testNoDeletionProtection() resource.TestCheckFunc {
	return resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_deletion_protection", "false")
}

func testAccVirtualClusterResource_removeDeletionProtection(vcNameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  configuration = {
    enable_deletion_protection = false
  }
}`, vcNameSuffix)
}

func testAccVirtualClusterResource_withRenamedCluster(vcNameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_renamed_%s"
  tier = "fundamentals"
  configuration = {
    enable_deletion_protection = false
  }
}`, vcNameSuffix)
}

func testAccVirtualClusterResource(vcNameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
}`, vcNameSuffix)
}

func testAccVirtualClusterResource_withPartialConfiguration(
	acls bool,
	vcNameSuffix string,
) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  configuration = {
    enable_acls = %t
  }
}`, vcNameSuffix, acls)
}

func testAccVirtualClusterResource_withConfiguration(
	acls bool,
	autoTopic bool,
	numParts int64,
	vcNameSuffix string,
) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  configuration = {
    enable_acls = %t
    default_num_partitions = %d
    auto_create_topic = %t
    enable_deletion_protection = true
  }
  tags = {
    "test_tag" = "test_value"
  }
}`, vcNameSuffix, acls, numParts, autoTopic)
}

func testAccVirtualClusterResourceCheck(acls bool, autoTopic bool, numParts int64, vcType string, tags bool, deletionProtection bool) resource.TestCheckFunc {
	var checks = []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "created_at"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "default", "false"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "type", vcType),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_acls", fmt.Sprintf("%t", acls)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.auto_create_topic", fmt.Sprintf("%t", autoTopic)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_num_partitions", fmt.Sprintf("%d", numParts)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_retention_millis", fmt.Sprintf("%d", 86400000)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "cloud.provider", "aws"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "cloud.region", "us-east-1"),
		// Note: agent_pool_name is now equal to "apn_test_acc_"+nameSuffix + randomSuffix
		utils.TestCheckResourceAttrStartsWith("warpstream_virtual_cluster.test", "agent_pool_name", "apn_test_acc_"),
		utils.TestCheckResourceAttrStartsWith("warpstream_virtual_cluster.test", "workspace_id", "wi_"),
	}

	if vcType == "byoc" {
		checks = append(checks,
			resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "agent_keys.#", "0"),
			utils.TestCheckResourceAttrMatchesRegex("warpstream_virtual_cluster.test", "bootstrap_url", `kafka\.discoveryv2\..+\.us-east-1\.warpstream\.com:9092`),
		)
	}
	if tags {
		checks = append(checks,
			resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "tags.test_tag", "test_value"),
		)
	}
	if deletionProtection {
		checks = append(checks,
			resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_deletion_protection", "true"),
		)
	}

	return resource.ComposeAggregateTestCheckFunc(checks...)

}

func TestAccVirtualClusterImport(t *testing.T) {
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

func TestAccVirtualClusterResourceWithSoftDeletion(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterResource_withSoftDeletionSettings(vcNameSuffix, false, 48),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_soft_topic_deletion", "false"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.soft_delete_topic_ttl_hours", "48"),
				),
			},
			{
				Config: testAccVirtualClusterResource_withSoftDeletionSettings(vcNameSuffix, true, 72),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_soft_topic_deletion", "true"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.soft_delete_topic_ttl_hours", "72"),
				),
			},
		},
	})
}

func testAccVirtualClusterResource_withSoftDeletionSettings(vcNameSuffix string, softDeleteEnable bool, ttlHours int64) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  configuration = {
    enable_soft_topic_deletion   = %t
    soft_delete_topic_ttl_hours  = %d
  }
}`, vcNameSuffix, softDeleteEnable, ttlHours)
}
