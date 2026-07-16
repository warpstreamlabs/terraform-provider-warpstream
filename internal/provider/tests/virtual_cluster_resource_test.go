package tests

import (
	"fmt"
	"regexp"
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
				Check:  testAccVirtualClusterResourceCheck(false, false, true, 1, "byoc", false, false),
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
				Check:  testAccVirtualClusterResourceCheck(false, false, true, 1, "byoc", false, false),
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
				Config: testAccVirtualClusterResource_withConfiguration(true, false, false, 2, vcNameSuffix),
				Check:  testAccVirtualClusterResourceCheck(true, false, false, 2, "byoc", true, true),
			},
			// Enable ACL shadowing
			{
				Config: testAccVirtualClusterResource_withConfiguration(false, true, false, 2, vcNameSuffix),
				Check:  testAccVirtualClusterResourceCheck(false, true, false, 2, "byoc", true, true),
			},
			// ACL shadowing and ACLs enabled should be mutually exclusive
			{
				Config:      testAccVirtualClusterResource_withConfiguration(true, true, false, 2, vcNameSuffix),
				ExpectError: regexp.MustCompile("enable_acls and enable_acl_shadowing cannot both be true"),
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
					testAccVirtualClusterResourceCheck(false, false, true, 1, "byoc", false, false),
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
	aclShadowing bool,
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
	enable_acl_shadowing = %t
    default_num_partitions = %d
    auto_create_topic = %t
    enable_deletion_protection = true
  }
  tags = {
    "test_tag" = "test_value"
  }
}`, vcNameSuffix, acls, aclShadowing, numParts, autoTopic)
}

func testAccVirtualClusterResourceCheck(acls bool, aclShadowing bool, autoTopic bool, numParts int64, vcType string, tags bool, deletionProtection bool) resource.TestCheckFunc {
	var checks = []resource.TestCheckFunc{
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttrSet("warpstream_virtual_cluster.test", "created_at"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "default", "false"),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "type", vcType),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_acls", fmt.Sprintf("%t", acls)),
		resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_acl_shadowing", fmt.Sprintf("%t", aclShadowing)),
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

func TestAccVirtualClusterResourceGenericConfig(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with two generic configs.
			{
				Config: testAccVirtualClusterResource_withGenericConfig(vcNameSuffix, "1048576"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "broker_configuration.%", "2"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "broker_configuration.message.max.bytes", "1048576"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "broker_configuration.delete.topic.enable", "true"),
				),
			},
			// Re-apply identical config: expect no drift.
			{
				Config: testAccVirtualClusterResource_withGenericConfig(vcNameSuffix, "1048576"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Update a value.
			{
				Config: testAccVirtualClusterResource_withGenericConfig(vcNameSuffix, "2097152"),
				Check:  resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "broker_configuration.message.max.bytes", "2097152"),
			},
			// Remove the generic config map entirely.
			{
				Config: testAccVirtualClusterResource(vcNameSuffix),
				Check:  resource.TestCheckNoResourceAttr("warpstream_virtual_cluster.test", "broker_configuration.message.max.bytes"),
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigTypedOverlap sets a setting that also has a typed
// attribute (retention) via the map, and verifies the typed attribute reflects the same
// value (ascribed from the API) and that a re-apply shows no drift.
func TestAccVirtualClusterResourceBrokerConfigTypedOverlap(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with retention set via the map.
			{
				Config: testAccVirtualClusterResource_withRetentionInMap(vcNameSuffix, "3600000"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "broker_configuration.log.retention.ms", "3600000"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_retention_millis", "3600000"),
				),
			},
			// Re-apply identical config: expect no drift.
			{
				Config: testAccVirtualClusterResource_withRetentionInMap(vcNameSuffix, "3600000"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Change the value; both the map and the typed attribute should reflect it.
			{
				Config: testAccVirtualClusterResource_withRetentionInMap(vcNameSuffix, "7200000"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "broker_configuration.log.retention.ms", "7200000"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_retention_millis", "7200000"),
				),
			},
		},
	})
}

// TestAccVirtualClusterResourceGenericConfigConflict verifies that setting the same
// underlying setting via both a typed attribute and the map is rejected at plan time by
// ModifyPlan (this check runs without a backend round-trip).
func TestAccVirtualClusterResourceGenericConfigConflict(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVirtualClusterResource_withConflictingConfig(vcNameSuffix),
				ExpectError: regexp.MustCompile("Conflicting virtual cluster configuration"),
			},
		},
	})
}

// TestAccVirtualClusterResourceRetentionAliasRejected verifies the retention ms-only rule:
// the minutes/hours aliases are rejected at plan time to avoid drift.
func TestAccVirtualClusterResourceRetentionAliasRejected(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccVirtualClusterResource_withRetentionAlias(vcNameSuffix),
				ExpectError: regexp.MustCompile("specify retention as `log.retention.ms`"),
			},
		},
	})
}

func testAccVirtualClusterResource_withGenericConfig(vcNameSuffix, messageMaxBytes string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  broker_configuration = {
    "message.max.bytes"   = "%s"
    "delete.topic.enable" = "true"
  }
}`, vcNameSuffix, messageMaxBytes)
}

func testAccVirtualClusterResource_withRetentionInMap(vcNameSuffix, retentionMillis string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  broker_configuration = {
    "log.retention.ms" = "%s"
  }
}`, vcNameSuffix, retentionMillis)
}

func testAccVirtualClusterResource_withRetentionAlias(vcNameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  broker_configuration = {
    "log.retention.hours" = "24"
  }
}`, vcNameSuffix)
}

func testAccVirtualClusterResource_withConflictingConfig(vcNameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  configuration = {
    default_retention_millis = 86400000
  }
  broker_configuration = {
    "log.retention.ms" = "86400000"
  }
}`, vcNameSuffix)
}

func TestAccVirtualClusterImport(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterResource_withDefaultTopicType(vcNameSuffix, "classic"),
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
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.soft_topic_deletion_ttl_millis", "172800000"),
				),
			},
			{
				Config: testAccVirtualClusterResource_withSoftDeletionSettings(vcNameSuffix, true, 72),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.enable_soft_topic_deletion", "true"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.soft_topic_deletion_ttl_millis", "259200000"),
				),
			},
		},
	})
}

func testAccVirtualClusterResource_withSoftDeletionSettings(vcNameSuffix string, softDeleteEnable bool, ttlHours int64) string {
	// Convert hours to milliseconds
	ttlMillis := ttlHours * 3600 * 1000
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  configuration = {
    enable_soft_topic_deletion   = %t
    soft_topic_deletion_ttl_millis  = %d
  }
}`, vcNameSuffix, softDeleteEnable, ttlMillis)
}

func TestAccVirtualClusterResourceWithDefaultTopicType(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create without default_topic_type (should be null)
			{
				Config: testAccVirtualClusterResource(vcNameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("warpstream_virtual_cluster.test", "configuration.default_topic_type"),
				),
			},
			// Test invalid value is rejected
			{
				Config:      testAccVirtualClusterResource_withDefaultTopicType(vcNameSuffix, "invalid"),
				ExpectError: regexp.MustCompile("Attribute configuration.default_topic_type value must be one of"),
			},
			// Update to set default_topic_type to "classic"
			{
				Config: testAccVirtualClusterResource_withDefaultTopicType(vcNameSuffix, "classic"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_topic_type", "classic"),
				),
			},
			// // Update to set default_topic_type to "lightning"
			{
				Config: testAccVirtualClusterResource_withDefaultTopicType(vcNameSuffix, "lightning"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "configuration.default_topic_type", "lightning"),
				),
			},
		},
	})
}

func testAccVirtualClusterResource_withDefaultTopicType(vcNameSuffix string, topicType string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  configuration = {
    default_topic_type = "%s"
  }
}`, vcNameSuffix, topicType)
}

func TestAccVirtualClusterResourceWithEvents(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with events disabled (explicit)
			{
				Config: testAccVirtualClusterResource_withEvents(vcNameSuffix, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.enabled", "false"),
				),
			},
			// Update to enable events
			{
				Config: testAccVirtualClusterResource_withEvents(vcNameSuffix, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.enabled", "true"),
				),
			},
			// Update to disable events
			{
				Config: testAccVirtualClusterResource_withEvents(vcNameSuffix, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.enabled", "false"),
				),
			},
		},
	})
}

func TestAccVirtualClusterResourceWithEventsDefault(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create without events block - should default to disabled
			{
				Config: testAccVirtualClusterResource(vcNameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.enabled", "false"),
				),
			},
		},
	})
}

func testAccVirtualClusterResource_withEvents(vcNameSuffix string, eventsEnabled bool) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  events = {
    enabled = %t
  }
}`, vcNameSuffix, eventsEnabled)
}

func TestAccVirtualClusterResourceWithEventTypes(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with event types configured
			{
				Config: testAccVirtualClusterResource_withEventTypes(vcNameSuffix, true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.enabled", "true"),
					// Check agent_logs
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.agent_logs.enabled", "true"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.agent_logs.retention_period_nanos", "604800000000000"),
					// Check pipeline_logs
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.pipeline_logs.enabled", "true"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.pipeline_logs.retention_period_nanos", "259200000000000"),
					// Verify acl_logs is not in state. Only configured event types should appear.
					resource.TestCheckNoResourceAttr("warpstream_virtual_cluster.test", "events.event_types.acl_logs"),
				),
			},
			// Update event types configuration
			{
				Config: testAccVirtualClusterResource_withEventTypes(vcNameSuffix, false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.enabled", "true"),
					// Check agent_logs is now disabled
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.agent_logs.enabled", "false"),
					// Check pipeline_logs is still enabled
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.pipeline_logs.enabled", "true"),
				),
			},
			// Disable all events
			{
				Config: testAccVirtualClusterResource_withEventTypes(vcNameSuffix, false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.enabled", "false"),
				),
			},
		},
	})
}

func TestAccVirtualClusterResourceEventTypesAllTypes(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with all three event types configured
			{
				Config: testAccVirtualClusterResource_withAllEventTypes(vcNameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.enabled", "true"),
					// Check all three event types are present
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.agent_logs.enabled", "true"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.pipeline_logs.enabled", "true"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.acl_logs.enabled", "true"),
				),
			},
			// Remove one event type from config
			{
				Config: testAccVirtualClusterResource_withEventTypes(vcNameSuffix, true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Only agent_logs and pipeline_logs should be in state now
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.agent_logs.enabled", "true"),
					resource.TestCheckResourceAttr("warpstream_virtual_cluster.test", "events.event_types.pipeline_logs.enabled", "true"),
					// acl_logs should not be in state
					resource.TestCheckNoResourceAttr("warpstream_virtual_cluster.test", "events.event_types.acl_logs"),
				),
			},
		},
	})
}

func testAccVirtualClusterResource_withEventTypes(vcNameSuffix string, agentLogsEnabled bool, pipelineLogsEnabled bool) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  events = {
    enabled = %t
    event_types = {
      agent_logs = {
        enabled                = %t
        retention_period_nanos = 604800000000000
      }
      pipeline_logs = {
        enabled                = %t
        retention_period_nanos = 259200000000000
      }
    }
  }
}`, vcNameSuffix, agentLogsEnabled || pipelineLogsEnabled, agentLogsEnabled, pipelineLogsEnabled)
}

func testAccVirtualClusterResource_withAllEventTypes(vcNameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  events = {
    enabled = true
    event_types = {
      agent_logs = {
        enabled                = true
        retention_period_nanos = 604800000000000
      }
      pipeline_logs = {
        enabled                = true
        retention_period_nanos = 259200000000000
      }
      acl_logs = {
        enabled                = true
        retention_period_nanos = 432000000000000
      }
    }
  }
}`, vcNameSuffix)
}
