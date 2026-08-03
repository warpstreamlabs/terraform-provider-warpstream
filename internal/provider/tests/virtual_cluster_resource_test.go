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

// emptyPlanChecks asserts a step's plan is a no-op, which is how every "does this settle?"
// requirement in this section is expressed.
var emptyPlanChecks = resource.ConfigPlanChecks{
	PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
}

// releasedProvider pins the published provider used by the upgrade tests. Bump both callers
// together when a newer release becomes the meaningful "before" state.
var releasedProvider = map[string]resource.ExternalProvider{
	"warpstream": {Source: "warpstreamlabs/warpstream", VersionConstraint: "2.7.9"},
}

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
// before calling the API: settings owned by a typed `configuration` attribute, write-only
// aliases (which redirect to the typed attribute when the setting has one), and null values.
// None of these steps reach the backend, so they are cheap enough to keep in one table.
func TestAccVirtualClusterResourceBrokerConfigInvalid(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	cases := []struct {
		name       string
		typedBody  string
		brokerBody string
		wantErr    string
	}{
		// The map and the typed attributes are disjoint: a setting with a typed attribute is
		// rejected in the map, and the error names the attribute to use.
		{
			name:       "typed-owned setting is rejected with a pointer",
			brokerBody: `    "log.retention.ms" = "3600000"`,
			wantErr:    `controlled\s+by\s+the\s+.configuration\.default_retention_millis.\s+attribute`,
		},
		{
			name:       "typed-owned topic type is rejected with a pointer",
			brokerBody: `    "warpstream.default.topic.type" = "lightning"`,
			wantErr:    `controlled\s+by\s+the\s+.configuration\.default_topic_type.\s+attribute`,
		},
		{
			// Declaring the typed attribute too does not change the answer: the map key is
			// rejected regardless, so the two surfaces can never overlap.
			name:       "typed-owned setting rejected even when the typed attribute agrees",
			typedBody:  `    default_retention_millis = 3600000`,
			brokerBody: `    "log.retention.ms" = "3600000"`,
			wantErr:    `controlled\s+by\s+the\s+.configuration\.default_retention_millis.\s+attribute`,
		},
		// Aliases for typed-owned settings redirect to the typed attribute, not to the
		// canonical key, which the map also rejects.
		{
			name:       "retention hours alias redirects to the typed attribute",
			brokerBody: `    "log.retention.hours" = "24"`,
			wantErr:    `alternate\s+unit\s+for\s+"log\.retention\.ms"[\s\S]*configuration\.default_retention_millis`,
		},
		{
			name:       "soft delete ttl hours alias redirects to the typed attribute",
			brokerBody: `    "warpstream.soft.delete.topic.ttl.hours" = "48"`,
			wantErr:    `alternate\s+unit\s+for[\s\S]*configuration\.soft_topic_deletion_ttl_millis`,
		},
		{
			name:       "null value cannot be tracked",
			brokerBody: `    "message.max.bytes" = null`,
			wantErr:    `null\s+is\s+not\s+a\s+valid\s+value`,
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
			// An empty value parses as a valid map entry, so it reaches the API, which rejects
			// it while parsing the config's value. The error arrives as raw JSON, so the quotes
			// around the config name are backslash-escaped.
			{
				Config:      brokerConfigResource(vcNameSuffix, "", `    "message.max.bytes" = ""`),
				ExpectError: regexp.MustCompile(`invalid\s+cluster\s+config\s+\\?"message\.max\.bytes\\?"`),
			},
			// An empty key is just an unsupported config name.
			{
				Config:      brokerConfigResource(vcNameSuffix, "", `    "" = "1048576"`),
				ExpectError: regexp.MustCompile(`unsupported\s+cluster\s+config\s+\\?"\\?"`),
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigLifecycle walks the create/update/remove cycle for
// configs that have no typed `configuration` equivalent, which is the majority of the surface
// and the plain case with no typed attribute involved.
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
				ConfigPlanChecks: emptyPlanChecks,
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
				ConfigPlanChecks: emptyPlanChecks,
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
				ConfigPlanChecks: emptyPlanChecks,
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigUpgrade is the backwards-compatibility guard. A
// configuration written against the released provider, which has no `broker_configuration`
// attribute at all, must plan clean once this provider takes over. This protects every existing
// user who never adopts the feature.
//
// Every setting whose wire representation this change moves to broker_configs is exercised,
// because each one is a chance to translate the value wrongly: the soft-delete TTL in
// particular used to be sent as a nanosecond duration under its own field and is now
// milliseconds under `warpstream.soft.delete.topic.ttl.ms`.
func TestAccVirtualClusterResourceBrokerConfigUpgrade(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	// A configuration the released provider understands: typed attributes, no map.
	config := brokerConfigResource(vcNameSuffix, `    auto_create_topic              = false
    default_num_partitions         = 4
    default_retention_millis       = 3600000
    default_topic_type             = "lightning"
    enable_soft_topic_deletion     = true
    soft_topic_deletion_ttl_millis = 172800000
    enable_acls                    = true`, "")

	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: releasedProvider,
				Config:            config,
			},
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Config:                   config,
				ConfigPlanChecks:         emptyPlanChecks,
			},
			// Taking over is not enough: the new provider must also be able to write these
			// settings through their new representation without changing what they mean.
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Config: brokerConfigResource(vcNameSuffix, `    auto_create_topic              = false
    default_num_partitions         = 4
    default_retention_millis       = 7200000
    default_topic_type             = "lightning"
    enable_soft_topic_deletion     = true
    soft_topic_deletion_ttl_millis = 259200000
    enable_acls                    = true`, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "7200000"),
					resource.TestCheckResourceAttr(addr, "configuration.soft_topic_deletion_ttl_millis", "259200000"),
					resource.TestCheckResourceAttr(addr, "configuration.default_topic_type", "lightning"),
				),
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigTypedSettings pins the typed-only path for the six
// settings the map rejects: all of them must round-trip (their wire representation is now the
// generic broker_configs field, so this also guards that translation) and a re-apply must plan
// nothing. Deleting a typed attribute reverts the cluster to the schema default; that
// pre-existing semantic is pinned here too.
func TestAccVirtualClusterResourceBrokerConfigTypedSettings(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	allSixTyped := `    auto_create_topic              = false
    default_num_partitions         = 4
    default_retention_millis       = 3600000
    enable_soft_topic_deletion     = false
    soft_topic_deletion_ttl_millis = 172800000
    default_topic_type             = "lightning"`

	fiveTyped := `    auto_create_topic              = false
    default_retention_millis       = 3600000
    enable_soft_topic_deletion     = false
    soft_topic_deletion_ttl_millis = 172800000
    default_topic_type             = "lightning"`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: brokerConfigResource(vcNameSuffix, allSixTyped, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "configuration.auto_create_topic", "false"),
					resource.TestCheckResourceAttr(addr, "configuration.default_num_partitions", "4"),
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "3600000"),
					resource.TestCheckResourceAttr(addr, "configuration.enable_soft_topic_deletion", "false"),
					resource.TestCheckResourceAttr(addr, "configuration.soft_topic_deletion_ttl_millis", "172800000"),
					resource.TestCheckResourceAttr(addr, "configuration.default_topic_type", "lightning"),
					resource.TestCheckNoResourceAttr(addr, "broker_configuration.%"),
				),
			},
			{
				Config:           brokerConfigResource(vcNameSuffix, allSixTyped, ""),
				ConfigPlanChecks: emptyPlanChecks,
			},
			// Delete default_num_partitions: the schema default reasserts and the cluster
			// reverts to 1.
			{
				Config: brokerConfigResource(vcNameSuffix, fiveTyped, ""),
				Check:  resource.TestCheckResourceAttr(addr, "configuration.default_num_partitions", "1"),
			},
			{
				Config:           brokerConfigResource(vcNameSuffix, fiveTyped, ""),
				ConfigPlanChecks: emptyPlanChecks,
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigCoexist verifies the two disjoint surfaces work side
// by side on one cluster: typed attributes own their settings, the map owns the rest, and each
// surface changes independently with settled plans in between.
func TestAccVirtualClusterResourceBrokerConfigCoexist(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	both := func(retention, maxBytes string) string {
		return brokerConfigResource(vcNameSuffix,
			"    default_retention_millis = "+retention,
			`    "message.max.bytes" = "`+maxBytes+`"`)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: both("3600000", "1048576"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "3600000"),
					resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "1048576"),
				),
			},
			{Config: both("3600000", "1048576"), ConfigPlanChecks: emptyPlanChecks},
			// Change only the typed side.
			{
				Config: both("7200000", "1048576"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "7200000"),
					resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "1048576"),
				),
			},
			// Change only the map side.
			{
				Config: both("7200000", "2097152"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "7200000"),
					resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "2097152"),
				),
			},
			{Config: both("7200000", "2097152"), ConfigPlanChecks: emptyPlanChecks},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigUnknownValue covers a map value that is not known
// until apply, which is what happens whenever a config is derived from another resource.
// Extracting the map must not fail at plan time and the plan must settle afterwards.
func TestAccVirtualClusterResourceBrokerConfigUnknownValue(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	// The dependency cluster's id is unknown until it is created, so the value derived from it
	// is too. The exact number does not matter; that it lands and the plan settles does.
	config := providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "dep" {
  name = "vcn_test_acc_%s_dep"
  tier = "dev"
}

resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"
  broker_configuration = {
    "message.max.bytes" = tostring(1000000 + length(warpstream_virtual_cluster.dep.id))
  }
}`, vcNameSuffix, vcNameSuffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttrSet(addr, "broker_configuration.message.max.bytes"),
			},
			{
				Config:           config,
				ConfigPlanChecks: emptyPlanChecks,
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigWholeMapUnknown covers a `broker_configuration` that
// is unknown as a whole, on both the create and the update path, plus the late-validation case:
// key validation cannot run against an opaque map at plan time, so a typed-owned key hiding in
// one must still be rejected during the apply-time re-plan, before anything is written.
func TestAccVirtualClusterResourceBrokerConfigWholeMapUnknown(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"

	// The JSON string interpolates the dependency cluster's id, so the whole decoded map is
	// unknown at plan time. The dep parameter picks which dependency feeds the map, so the
	// update step can make the map opaque again by deriving it from a dependency that does not
	// exist yet.
	config := func(dep, key string, base int) string {
		return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "%[1]s" {
  name = "vcn_test_acc_%[2]s_%[1]s"
  tier = "dev"
}

locals {
  encoded_%[2]s = jsonencode({
    %[3]q = tostring(%[4]d + length(warpstream_virtual_cluster.%[1]s.id))
  })
}

resource "warpstream_virtual_cluster" "test" {
  name                 = "vcn_test_acc_%[2]s"
  tier                 = "fundamentals"
  broker_configuration = jsondecode(local.encoded_%[2]s)
}`, dep, vcNameSuffix, key, base)
	}

	expectMapUnknown := resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectUnknownValue(addr, tfjsonpath.New("broker_configuration")),
		},
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with a wholly-unknown map.
			{
				Config:           config("dep", "message.max.bytes", 1000000),
				ConfigPlanChecks: expectMapUnknown,
				Check:            resource.TestCheckResourceAttrSet(addr, "broker_configuration.message.max.bytes"),
			},
			{Config: config("dep", "message.max.bytes", 1000000), ConfigPlanChecks: emptyPlanChecks},
			// Update with a wholly-unknown map: rekey it off a new dependency cluster.
			{
				Config:           config("dep2", "message.max.bytes", 2000000),
				ConfigPlanChecks: expectMapUnknown,
				Check:            resource.TestCheckResourceAttrSet(addr, "broker_configuration.message.max.bytes"),
			},
			{Config: config("dep2", "message.max.bytes", 2000000), ConfigPlanChecks: emptyPlanChecks},
			// A typed-owned key hiding in an opaque map: plan-time validation cannot see it, so
			// the rejection must fire during the apply-time re-plan instead.
			{
				Config:      config("dep3", "log.retention.ms", 3600000),
				ExpectError: regexp.MustCompile(`controlled\s+by\s+the\s+.configuration\.default_retention_millis.\s+attribute`),
			},
			// The cluster and state must still be recoverable afterwards. The failed step did
			// real work before the rejection (dep3 created, dep2 destroyed), so this step
			// legitimately applies; the one after must then plan nothing.
			{
				Config: config("dep2", "message.max.bytes", 2000000),
				Check:  resource.TestCheckResourceAttrSet(addr, "broker_configuration.message.max.bytes"),
			},
			{Config: config("dep2", "message.max.bytes", 2000000), ConfigPlanChecks: emptyPlanChecks},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigLargeRetention covers a retention longer than an
// int32 of milliseconds can hold (30 days is 2,592,000,000 ms, past the 2,147,483,647 limit).
// Retention is set through its typed attribute, but the wire representation is the generic
// broker_configs field, so this guards that log.retention.ms stays 64-bit end to end.
func TestAccVirtualClusterResourceBrokerConfigLargeRetention(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"
	const thirtyDaysMillis = "2592000000"

	config := brokerConfigResource(vcNameSuffix, "    default_retention_millis = "+thirtyDaysMillis, "")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", thirtyDaysMillis),
			},
			{
				Config:           config,
				ConfigPlanChecks: emptyPlanChecks,
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigDrift is the read path's central claim: declared keys
// track the cluster, undeclared ones are invisible. Both halves matter — the first is what makes
// the attribute usable, the second is why a config someone sets outside Terraform (or through a
// typed attribute) never shows up as churn on an unrelated cluster.
func TestAccVirtualClusterResourceBrokerConfigDrift(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"
	config := brokerConfigResource(vcNameSuffix, "", `    "message.max.bytes" = "1048576"`)

	// The cluster id only exists once the first step has applied, so capture it from state and
	// let the later steps reach past Terraform to change the cluster behind its back.
	var clusterID string
	writeOutOfBand := func(kv map[string]string) func() {
		return func() {
			require.NoError(t, client.UpdateConfiguration(
				api.ConfigurationUpdate{BrokerConfigs: kv},
				api.VirtualCluster{ID: clusterID},
			))
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "1048576"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources[addr]
						if !ok {
							return fmt.Errorf("resource %s not found in state", addr)
						}
						clusterID = rs.Primary.Attributes["id"]
						if clusterID == "" {
							return fmt.Errorf("resource %s has no id in state", addr)
						}
						return nil
					},
				),
			},
			// A config the configuration never mentioned must not surface as drift.
			{
				PreConfig:        writeOutOfBand(map[string]string{"offsets.retention.minutes": "20160"}),
				Config:           config,
				ConfigPlanChecks: emptyPlanChecks,
			},
			// A declared config changed behind Terraform's back must surface as drift, and the
			// apply must put the configured value back.
			{
				PreConfig: writeOutOfBand(map[string]string{"message.max.bytes": "2097152"}),
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "1048576"),
			},
			{Config: config, ConfigPlanChecks: emptyPlanChecks},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigUpgradeDefaults is the other half of the upgrade
// guard, and the more common configuration by far: name and tier only, every `configuration`
// attribute left at its default, `default_topic_type` never set. The released provider sent those
// defaults as typed request fields and this one sends them as broker configs, so taking such a
// cluster over must plan clean — and must not invent a topic type where the user never chose one.
func TestAccVirtualClusterResourceBrokerConfigUpgradeDefaults(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"
	config := brokerConfigResource(vcNameSuffix, "", "")

	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: releasedProvider,
				Config:            config,
			},
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Config:                   config,
				ConfigPlanChecks:         emptyPlanChecks,
			},
			// Planning clean is not enough: the defaults must still be the defaults once this
			// provider has written them through their new representation.
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Config:                   config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "configuration.auto_create_topic", "true"),
					resource.TestCheckResourceAttr(addr, "configuration.default_num_partitions", "1"),
					resource.TestCheckResourceAttr(addr, "configuration.default_retention_millis", "86400000"),
					resource.TestCheckResourceAttr(addr, "configuration.enable_soft_topic_deletion", "true"),
					resource.TestCheckResourceAttr(addr, "configuration.soft_topic_deletion_ttl_millis", "86400000"),
					resource.TestCheckResourceAttr(addr, "configuration.enable_acls", "false"),
					resource.TestCheckNoResourceAttr(addr, "configuration.default_topic_type"),
					resource.TestCheckNoResourceAttr(addr, "broker_configuration.%"),
				),
			},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigFailureRecovery covers both ways an apply can be
// rejected by the API — during the create that follows a fresh cluster, and during a later update
// — and requires the resource to be recoverable from each.
//
// The final empty plan is the load-bearing assertion. A failure path that writes partial state
// persists whatever unknowns the plan was carrying, and `events.event_types` is one of them; once
// that lands as null it is re-planned as "known after apply" forever, so a single rejected value
// would leave the resource with a permanent diff no apply can clear.
func TestAccVirtualClusterResourceBrokerConfigFailureRecovery(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"
	good := brokerConfigResource(vcNameSuffix, "", `    "message.max.bytes" = "1048576"`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// The cluster is created and then fails to configure.
			{
				Config:      brokerConfigResource(vcNameSuffix, "", `    "nope.not.a.config" = "1"`),
				ExpectError: regexp.MustCompile(`unsupported\s+cluster\s+config`),
			},
			{
				Config: good,
				Check:  resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "1048576"),
			},
			{Config: good, ConfigPlanChecks: emptyPlanChecks},
			// Now the same thing on the update path, from a healthy resource.
			{
				Config:      brokerConfigResource(vcNameSuffix, "", `    "message.max.bytes" = "not-a-number"`),
				ExpectError: regexp.MustCompile(`invalid\s+cluster\s+config`),
			},
			{
				Config: good,
				Check:  resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "1048576"),
			},
			{Config: good, ConfigPlanChecks: emptyPlanChecks},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigWithEvents pins the map alongside an events block.
// Events are the attribute a botched failure path corrupts first, because it is the one carrying
// unknowns through the apply, so the plain case with both populated is worth holding still.
func TestAccVirtualClusterResourceBrokerConfigWithEvents(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"
	config := providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test_acc_%s"
  tier = "fundamentals"

  broker_configuration = {
    "message.max.bytes" = "1048576"
  }

  events = {
    enabled = true
  }
}`, vcNameSuffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "broker_configuration.message.max.bytes", "1048576"),
					resource.TestCheckResourceAttr(addr, "events.enabled", "true"),
				),
			},
			{Config: config, ConfigPlanChecks: emptyPlanChecks},
		},
	})
}

// TestAccVirtualClusterResourceBrokerConfigImport covers importing a cluster whose configs are
// map-managed. Import has no configuration to learn the declared keys from, so it records none;
// what has to hold is that everything else round-trips and the follow-up plan settles rather than
// erroring or proposing a replace.
func TestAccVirtualClusterResourceBrokerConfigImport(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	const addr = "warpstream_virtual_cluster.test"
	config := brokerConfigResource(vcNameSuffix, `    default_retention_millis = 3600000`,
		`    "message.max.bytes" = "1048576"`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				Config:            config,
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				// broker_configuration is expected to differ, per the doc comment above.
				//
				// default_topic_type differs for an unrelated and pre-existing reason: a topic
				// type the configuration never set is null in applied state but "classic" after
				// import, because import has no prior state to learn "unset" from. Verified to
				// behave identically on the released provider, so it is not this attribute's
				// doing; it is ignored here to keep the assertion about broker configs.
				ImportStateVerifyIgnore: []string{"broker_configuration", "configuration.default_topic_type"},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported instance, got %d", len(states))
					}
					if v, ok := states[0].Attributes["broker_configuration.%"]; ok {
						return fmt.Errorf("imported state tracks %s broker configs, expected none", v)
					}
					return nil
				},
			},
			{Config: config, ConfigPlanChecks: emptyPlanChecks},
		},
	})
}

// TestAccVirtualClusterConfigSurfacesAreDisjoint checks the assumption the disjoint design rests
// on, straight against the API: the three settings the provider exposes only as typed attributes
// have no Kafka-style config name at all, so `broker_configuration` cannot reach them and the two
// surfaces cannot fight over one setting.
//
// If the API ever gains a name for one of these, this test fails — which is the signal to add it
// to typedAttrConfigs before the map can be used to write it behind the typed attribute's back.
func TestAccVirtualClusterConfigSurfacesAreDisjoint(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	region := "us-east-1"
	vc, err := client.CreateVirtualCluster(vcNameSuffix, api.ClusterParameters{
		Type:   api.VirtualClusterTypeBYOC,
		Tier:   api.VirtualClusterTierPro,
		Region: &region,
		Cloud:  "aws",
	})
	require.NoError(t, err)
	defer func() {
		if err := client.DeleteVirtualCluster(vc.ID, vc.Name); err != nil {
			panic(fmt.Errorf("failed to delete virtual cluster: %w", err))
		}
	}()

	for _, key := range []string{
		"warpstream.acls.enable",
		"warpstream.acl.shadowing.enable",
		"warpstream.deletion.protection.enable",
	} {
		err := client.UpdateConfiguration(api.ConfigurationUpdate{
			BrokerConfigs: map[string]string{key: "true"},
		}, *vc)
		require.ErrorContains(t, err, "unsupported cluster config",
			"the API now accepts %q as a broker config, so broker_configuration can write a "+
				"setting that also has a typed attribute; add it to typedAttrConfigs", key)
	}

	// Sanity check that the loop above proves something: a name the API does accept must work.
	require.NoError(t, client.UpdateConfiguration(api.ConfigurationUpdate{
		BrokerConfigs: map[string]string{"message.max.bytes": "1048576"},
	}, *vc))
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
