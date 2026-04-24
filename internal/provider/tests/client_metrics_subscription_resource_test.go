package tests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stretchr/testify/require"

	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

const cmsResourceAddr = "warpstream_client_metrics_subscription.test"

func testAccCMSConfigAllFields(vcRand string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
  name = "vcn_%s"
  tier = "dev"
}

resource "warpstream_client_metrics_subscription" "test" {
  virtual_cluster_id = warpstream_virtual_cluster.default.id
  name               = "producers"

  interval_ms = 60000
  metrics     = "org.apache.kafka.producer."
  match       = "client_id=^app-.*"
}
`, vcRand)
}

func testAccCMSConfigPartial(vcRand string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
  name = "vcn_%s"
  tier = "dev"
}

resource "warpstream_client_metrics_subscription" "test" {
  virtual_cluster_id = warpstream_virtual_cluster.default.id
  name               = "producers"

  interval_ms = 30000
  metrics     = "org.apache.kafka.consumer."
}
`, vcRand)
}

// TestAccClientMetricsSubscriptionResource exercises create, then a
// whole-config replace that drops the `match` field, verifying that omitted
// fields become null on the server (matches the controller behaviour proved
// by TestControllerClientMetricsSubscriptions_RoundTrip in warpstream).
func TestAccClientMetricsSubscriptionResource(t *testing.T) {
	vcRand := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCMSConfigAllFields(vcRand),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith(cmsResourceAddr, "virtual_cluster_id", "vci_"),
					utils.TestCheckResourceAttrStartsWith(cmsResourceAddr, "id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(cmsResourceAddr, tfjsonpath.New("name"), knownvalue.StringExact("producers")),
					statecheck.ExpectKnownValue(cmsResourceAddr, tfjsonpath.New("interval_ms"), knownvalue.Int64Exact(60000)),
					statecheck.ExpectKnownValue(cmsResourceAddr, tfjsonpath.New("metrics"), knownvalue.StringExact("org.apache.kafka.producer.")),
					statecheck.ExpectKnownValue(cmsResourceAddr, tfjsonpath.New("match"), knownvalue.StringExact("client_id=^app-.*")),
				},
			},
			// Whole-replace: drop `match`, change interval and metrics.
			{
				Config: testAccCMSConfigPartial(vcRand),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(cmsResourceAddr, tfjsonpath.New("interval_ms"), knownvalue.Int64Exact(30000)),
					statecheck.ExpectKnownValue(cmsResourceAddr, tfjsonpath.New("metrics"), knownvalue.StringExact("org.apache.kafka.consumer.")),
					statecheck.ExpectKnownValue(cmsResourceAddr, tfjsonpath.New("match"), knownvalue.Null()),
				},
			},
			// No-op replan should produce an empty plan.
			{
				Config: testAccCMSConfigPartial(vcRand),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Import: the composite ID is `<vc_id>/<name>`.
			{
				ResourceName:      cmsResourceAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccClientMetricsSubscriptionResource_RequiresReplace covers that
// changing `name` triggers a destroy/create rather than an update. Same goes
// for `virtual_cluster_id` (transitively, since the cluster will also be
// recreated).
func TestAccClientMetricsSubscriptionResource_RequiresReplace(t *testing.T) {
	vcRand := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	renamed := providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
  name = "vcn_%s"
  tier = "dev"
}

resource "warpstream_client_metrics_subscription" "test" {
  virtual_cluster_id = warpstream_virtual_cluster.default.id
  name               = "consumers"

  interval_ms = 60000
}
`, vcRand)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCMSConfigAllFields(vcRand),
			},
			{
				Config: renamed,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(cmsResourceAddr, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(cmsResourceAddr, tfjsonpath.New("name"), knownvalue.StringExact("consumers")),
				},
			},
		},
	})
}

// TestAccClientMetricsSubscriptionResource_ConfigValidation covers every
// schema-level validator so that invalid configurations fail at plan time
// instead of as an HTTP 400 during apply. Each case mirrors a specific
// branch of pkg/webby/controllers/controller_client_metrics.go and
// pkg/kafka/common/client_metrics.go: empty-config, interval bounds, and
// match pattern validity.
func TestAccClientMetricsSubscriptionResource_ConfigValidation(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		expectError *regexp.Regexp
	}{
		{
			name:        "empty_config",
			body:        ``,
			expectError: regexp.MustCompile(`(?i)at least one`),
		},
		{
			name:        "interval_below_min",
			body:        `interval_ms = 99`,
			expectError: regexp.MustCompile(`(?i)interval_ms`),
		},
		{
			name:        "interval_above_max",
			body:        `interval_ms = 3600001`,
			expectError: regexp.MustCompile(`(?i)interval_ms`),
		},
		{
			name:        "match_missing_equals",
			body:        `match = "client_id_no_equals"`,
			expectError: regexp.MustCompile(`(?i)illegal client matching pattern`),
		},
		{
			name:        "match_unknown_selector",
			body:        `match = "unknown_selector_key=foo"`,
			expectError: regexp.MustCompile(`(?i)unknown selector key`),
		},
		{
			name:        "match_invalid_regex",
			body:        `match = "client_id=[invalid"`,
			expectError: regexp.MustCompile(`(?i)not a valid regular expression`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vcRand := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
			config := providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
  name = "vcn_%s"
  tier = "dev"
}

resource "warpstream_client_metrics_subscription" "test" {
  virtual_cluster_id = warpstream_virtual_cluster.default.id
  name               = "producers"
  %s
}
`, vcRand, tc.body)

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      config,
						ExpectError: tc.expectError,
						PlanOnly:    true,
					},
				},
			})
		})
	}
}

// TestAccClientMetricsSubscriptionResource_DriftRecreate deletes the
// subscription out-of-band (mimicking another operator clearing it via the
// AdminClient path) and verifies the next plan recreates the resource. This
// confirms Read maps the API's 404 onto resource removal in state.
func TestAccClientMetricsSubscriptionResource_DriftRecreate(t *testing.T) {
	vcRand := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	vcName := fmt.Sprintf("vcn_%s", vcRand)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCMSConfigAllFields(vcRand),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith(cmsResourceAddr, "virtual_cluster_id", "vci_"),
				),
			},
			{
				PreConfig: func() {
					client, err := api.NewClientDefault()
					require.NoError(t, err)

					vc, err := client.FindVirtualCluster(vcName)
					require.NoError(t, err)

					require.NoError(t, client.DeleteClientMetricsSubscriptions(vc.ID, []string{"producers"}))
				},
				Config:             testAccCMSConfigAllFields(vcRand),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(cmsResourceAddr, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}
