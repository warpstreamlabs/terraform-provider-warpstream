package tests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
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

// --- broker_configuration (generic cluster config map) ---------------------------------
//
// The tests below are deliberately few and broad. Each one owns a theme and walks a cluster
// through several steps, rather than spreading one assertion per test across many clusters.

// brokerConfigResource renders a virtual cluster with an optional typed `configuration` body
// and an optional `broker_configuration` map, so a single fixture covers every combination the
// tests need.
func brokerConfigResource(vcNameSuffix, typedBody, brokerBody string) string {
	typed := ""
	if typedBody != "" {
		typed = fmt.Sprintf("  configuration = {\n%s\n  }\n", typedBody)
	}
	broker := ""
	if brokerBody != "" {
		broker = fmt.Sprintf("  broker_configuration = {\n%s\n  }\n", brokerBody)
	}
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
%s%s}`, vcNameSuffix, typed, broker)
}

// brokerConfigResourceEmptyMap renders the cluster with an explicitly empty
// `broker_configuration`.
func brokerConfigResourceEmptyMap(vcNameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name                 = "vcn_test_acc_%s"
  tier                 = "fundamentals"
  broker_configuration = {}
}`, vcNameSuffix)
}

// TestAccVirtualClusterResourceBrokerConfigInvalid covers every input the provider refuses
// before calling the API: unsupported names, write-only aliases, null values, values the API
// would silently rewrite, and one setting given two disagreeing values. None of these steps
// reach the backend, so they are cheap enough to keep in one table.
func TestAccVirtualClusterResourceBrokerConfigInvalid(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	cases := []struct {
		name       string
		typedBody  string
		brokerBody string
		wantErr    string
	}{
		{
			name:       "retention hours is a write-only alias",
			brokerBody: `    "log.retention.hours" = "24"`,
			wantErr:    `"log.retention.hours"\s+is\s+a\s+write-only\s+alias`,
		},
		{
			name:       "soft delete ttl hours is a write-only alias",
			brokerBody: `    "warpstream.soft.delete.topic.ttl.hours" = "48"`,
			wantErr:    `"warpstream.soft.delete.topic.ttl.hours"\s+is\s+a\s+write-only\s+alias`,
		},
		{
			name:       "null value cannot be tracked",
			brokerBody: `    "message.max.bytes" = null`,
			wantErr:    `null\s+is\s+not\s+a\s+valid\s+value`,
		},
		{
			name:       "same setting given two different values",
			typedBody:  `    default_retention_millis = 3600000`,
			brokerBody: `    "log.retention.ms" = "7200000"`,
			wantErr:    `Conflicting\s+virtual\s+cluster\s+configuration`,
		},
		{
			// The two surfaces cannot be compared, and the error has to name the key that could
			// not be read rather than just quoting the value.
			name:       "value that cannot be read as its typed twin's type",
			typedBody:  `    default_retention_millis = 3600000`,
			brokerBody: `    "log.retention.ms" = "1MB"`,
			wantErr:    `"log\.retention\.ms":\s+"1MB"\s+is\s+not\s+an\s+integer`,
		},
	}

	steps := make([]resource.TestStep, 0, len(cases))
	for _, c := range cases {
		steps = append(steps, resource.TestStep{
			Config:      brokerConfigResource(vcNameSuffix, c.typedBody, c.brokerBody),
			ExpectError: regexp.MustCompile(c.wantErr),
		})
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

// TestAccVirtualClusterResourceBrokerConfigRejectedByAPI covers the inputs the provider
// deliberately does not police, because doing so would mean hardcoding which config names exist
// and how each one's values are normalised — the knowledge that would need a provider release
// every time the API gains a config.
//
// An unsupported name is rejected by the API. A value the API rewrites is caught by the read
// that follows the write, which reports the exact value to use. Neither needs the provider to
// know anything about the config in question.
func TestAccVirtualClusterResourceBrokerConfigRejectedByAPI(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      brokerConfigResource(vcNameSuffix, "", `    "messge.max.bytes" = "1048576"`),
				ExpectError: regexp.MustCompile(`unsupported\s+cluster\s+config`),
			},
			{
				Config:      brokerConfigResource(vcNameSuffix, "", `    "delete.topic.enable" = "TRUE"`),
				ExpectError: regexp.MustCompile(`the\s+API\s+reports\s+it\s+as\s+"true"`),
			},
			{
				Config:      brokerConfigResource(vcNameSuffix, "", `    "log.retention.ms" = "-5"`),
				ExpectError: regexp.MustCompile(`the\s+API\s+reports\s+it\s+as\s+"-1"`),
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigLifecycle walks the create/update/remove cycle for
// configs that have no typed `configuration` equivalent, which is the majority of the surface
// and the plain case with no mirroring involved.
func TestAccVirtualClusterResourceBrokerConfigLifecycle(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	twoConfigs := `    "message.max.bytes"   = "1048576"
    "delete.topic.enable" = "true"`
	changedAndAdded := `    "message.max.bytes"         = "2097152"
    "delete.topic.enable"       = "true"
    "offsets.retention.minutes" = "10080"`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: brokerConfigResource(vcNameSuffix, "", twoConfigs),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "broker_configuration.%", "2"),
					resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "1048576"),
					resource.TestCheckResourceAttr(addr, "broker_configuration.delete.topic.enable", "true"),
				),
			},
			{
				Config:           brokerConfigResource(vcNameSuffix, "", twoConfigs),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			// Change one value and add a key the provider has never sent before.
			{
				Config: brokerConfigResource(vcNameSuffix, "", changedAndAdded),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "broker_configuration.%", "3"),
					resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "2097152"),
					resource.TestCheckResourceAttr(addr, "broker_configuration.offsets.retention.minutes", "10080"),
				),
			},
			// Removing the attribute drops the keys from state. The API has no way to revert a
			// config to its default, so the cluster keeps the values; this only asserts that
			// Terraform stops tracking them and that the plan settles.
			{
				Config: brokerConfigResource(vcNameSuffix, "", ""),
				Check:  resource.TestCheckNoResourceAttr(addr, "broker_configuration.%"),
			},
			{
				Config:           brokerConfigResource(vcNameSuffix, "", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			// An explicitly empty map is a distinct value from an absent attribute, and the
			// attribute is Optional rather than Computed, so it has to round-trip as empty. A
			// module writing `broker_configuration = var.configs` with a `{}` default lands here,
			// and reporting it back as null aborts the apply as an inconsistent result.
			{
				Config: brokerConfigResourceEmptyMap(vcNameSuffix),
				Check:  resource.TestCheckResourceAttr(addr, "broker_configuration.%", "0"),
			},
			{
				Config:           brokerConfigResourceEmptyMap(vcNameSuffix),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigMirrored covers every setting that exists in both
// the map and the deprecated typed `configuration` attribute, all at once. Each typed attribute
// must end up holding the value declared in the map, and a re-apply must plan nothing.
//
// This is where the interesting cases live: `default_topic_type` is the only mirrored attribute
// with no schema default and is force-nulled on read unless the map owns it, and retention and
// soft-delete TTL both collapse any negative value to "-1" meaning infinite, with the typed TTL
// reported in nanoseconds rather than milliseconds.
func TestAccVirtualClusterResourceBrokerConfigMirrored(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	allSix := `    "auto.create.topics.enable"           = "false"
    "num.partitions"                      = "4"
    "log.retention.ms"                    = "3600000"
    "warpstream.soft.delete.topic.enable" = "false"
    "warpstream.soft.delete.topic.ttl.ms" = "172800000"
    "warpstream.default.topic.type"       = "lightning"`

	infinite := `    "auto.create.topics.enable"           = "false"
    "num.partitions"                      = "4"
    "log.retention.ms"                    = "-1"
    "warpstream.soft.delete.topic.enable" = "false"
    "warpstream.soft.delete.topic.ttl.ms" = "-1"
    "warpstream.default.topic.type"       = "lightning"`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: brokerConfigResource(vcNameSuffix, "", allSix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "configuration.auto_create_topic", "false"),
					resource.TestCheckResourceAttr(addr, "configuration.default_num_partitions", "4"),
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "3600000"),
					resource.TestCheckResourceAttr(addr, "configuration.enable_soft_topic_deletion", "false"),
					resource.TestCheckResourceAttr(addr, "configuration.soft_topic_deletion_ttl_millis", "172800000"),
					resource.TestCheckResourceAttr(addr, "configuration.default_topic_type", "lightning"),
				),
			},
			{
				Config:           brokerConfigResource(vcNameSuffix, "", allSix),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			// Infinite retention and infinite soft-delete TTL. The typed TTL is reported by the
			// API as a 100-year duration rather than -1, so this also exercises the consistency
			// check that must not treat that as the API contradicting itself.
			{
				Config: brokerConfigResource(vcNameSuffix, "", infinite),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "broker_configuration.log.retention.ms", "-1"),
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "-1"),
					resource.TestCheckResourceAttr(addr, "broker_configuration.warpstream.soft.delete.topic.ttl.ms", "-1"),
				),
			},
			{
				Config:           brokerConfigResource(vcNameSuffix, "", infinite),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigMigration is the reason the map is allowed to
// overlap the deprecated typed attributes at all: a module must be able to adopt
// `broker_configuration` without deleting its typed attributes in the same change. Nothing
// changes on the cluster across these steps, so every plan after the first must be empty.
func TestAccVirtualClusterResourceBrokerConfigMigration(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	emptyPlan := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Where an existing configuration starts: typed attribute only.
			{
				Config: brokerConfigResource(vcNameSuffix, `    default_retention_millis = 3600000`, ""),
				Check:  resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "3600000"),
			},
			// Add the map alongside it with the same value. Nothing changes on the cluster, but
			// Terraform does start tracking a new attribute, so this step legitimately plans an
			// update; only the value matters here.
			{
				Config: brokerConfigResource(vcNameSuffix,
					`    default_retention_millis = 3600000`,
					`    "log.retention.ms" = "3600000"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "broker_configuration.log.retention.ms", "3600000"),
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "3600000"),
				),
			},
			// Both surfaces declared and settled: re-applying must plan nothing.
			{
				Config: brokerConfigResource(vcNameSuffix,
					`    default_retention_millis = 3600000`,
					`    "log.retention.ms" = "3600000"`),
				ConfigPlanChecks: emptyPlan,
			},
			// Drop the typed attribute, leaving the map in charge. The mirrored attribute keeps
			// its value, so this is a genuine no-op.
			{
				Config:           brokerConfigResource(vcNameSuffix, "", `    "log.retention.ms" = "3600000"`),
				ConfigPlanChecks: emptyPlan,
				Check:            resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "3600000"),
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigUpgrade is the backwards-compatibility guard. A
// configuration written against the released provider, which has no `broker_configuration`
// attribute at all, must plan clean once this provider takes over. This protects every existing
// user who never adopts the feature.
func TestAccVirtualClusterResourceBrokerConfigUpgrade(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	// A configuration the released provider understands: typed attributes, no map.
	config := brokerConfigResource(vcNameSuffix, `    default_retention_millis = 3600000
    enable_acls              = true`, "")

	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"warpstream": {Source: "warpstreamlabs/warpstream", VersionConstraint: "2.7.9"},
				},
				Config: config,
			},
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Config:                   config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigUnknownValue covers a value that is not known until
// apply, which is what happens whenever a config is derived from another resource. Extracting
// the map must not fail at plan time, and the mirrored typed attribute must plan as
// known-after-apply rather than keeping a schema default the apply is about to contradict.
func TestAccVirtualClusterResourceBrokerConfigUnknownValue(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	// The dependency cluster's id is unknown until it is created, so the retention derived from
	// it is too. The exact number does not matter; that the two surfaces agree afterwards does.
	config := providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "dep" {
  name = "vcn_test_acc_%s_dep"
  tier = "dev"
}

resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  broker_configuration = {
    "log.retention.ms" = tostring(length(warpstream_virtual_cluster.dep.id) * 100000)
  }
}`, vcNameSuffix, vcNameSuffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue(addr,
							tfjsonpath.New("configuration").AtMapKey("default_retention_millis")),
					},
				},
				Check: resource.TestCheckResourceAttrPair(
					addr, "broker_configuration.log.retention.ms",
					addr, "configuration.default_retention_millis",
				),
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigWholeMapUnknown covers a `broker_configuration` that
// is unknown as a whole, rather than one whose individual values are unknown.
func TestAccVirtualClusterResourceBrokerConfigWholeMapUnknown(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	// The dependency cluster's id is unknown until it is created, so the JSON string built from it
	// is unknown, and decoding it yields a map whose keys are not visible at plan time.
	config := providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "dep" {
  name = "vcn_test_acc_%s_dep"
  tier = "dev"
}

locals {
  encoded_%s = jsonencode({
    "log.retention.ms" = tostring(length(warpstream_virtual_cluster.dep.id) * 100000)
  })
}

resource "warpstream_virtual_cluster" "test" {
  name                 = "vcn_test_acc_%s"
  tier                 = "fundamentals"
  broker_configuration = jsondecode(local.encoded_%s)
}`, vcNameSuffix, vcNameSuffix, vcNameSuffix, vcNameSuffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						// The schema default must not stand here: the map is about to set this.
						plancheck.ExpectUnknownValue(addr,
							tfjsonpath.New("configuration").AtMapKey("default_retention_millis")),
					},
				},
				Check: resource.TestCheckResourceAttrPair(
					addr, "broker_configuration.log.retention.ms",
					addr, "configuration.default_retention_millis",
				),
			},
			{
				Config:           config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigLargeRetention covers a retention longer than an
// int32 of milliseconds can hold (30 days is 2,592,000,000 ms, past the 2,147,483,647 limit).
// The typed `default_retention_millis` attribute has always been a 64-bit integer, so this must
// work through the map too, otherwise long retention would regress for anyone migrating.
func TestAccVirtualClusterResourceBrokerConfigLargeRetention(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"
	const thirtyDaysMillis = "2592000000"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: brokerConfigResource(vcNameSuffix, "", `    "log.retention.ms" = "`+thirtyDaysMillis+`"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "broker_configuration.log.retention.ms", thirtyDaysMillis),
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", thirtyDaysMillis),
				),
			},
			{
				Config:           brokerConfigResource(vcNameSuffix, "", `    "log.retention.ms" = "`+thirtyDaysMillis+`"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
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
