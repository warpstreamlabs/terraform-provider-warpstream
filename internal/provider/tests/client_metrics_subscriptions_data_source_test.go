package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const cmsListAddr = "data.warpstream_client_metrics_subscriptions.test"

// TestAccClientMetricsSubscriptionsDataSource verifies that the list data
// source returns every subscription in a Virtual Cluster, sorted by name
// (the server sorts via handleListClientMetricsSubscriptions).
func TestAccClientMetricsSubscriptionsDataSource(t *testing.T) {
	vcRand := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	cfg := providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
  name = "vcn_%s"
  tier = "dev"
}

resource "warpstream_client_metrics_subscription" "producers" {
  virtual_cluster_id = warpstream_virtual_cluster.default.id
  name               = "producers"

  interval_ms = 60000
  metrics     = "org.apache.kafka.producer."
  match       = "client_id=^app-.*"
}

resource "warpstream_client_metrics_subscription" "consumers" {
  virtual_cluster_id = warpstream_virtual_cluster.default.id
  name               = "consumers"

  interval_ms = 30000
  metrics     = "org.apache.kafka.consumer."
}

data "warpstream_client_metrics_subscriptions" "test" {
  virtual_cluster_id = warpstream_virtual_cluster.default.id

  depends_on = [
    warpstream_client_metrics_subscription.producers,
    warpstream_client_metrics_subscription.consumers,
  ]
}
`, vcRand)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(cmsListAddr, "virtual_cluster_id"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions"), knownvalue.ListSizeExact(2)),

					// Server sorts by name: "consumers" (0) before "producers" (1).
					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("consumers")),
					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions").AtSliceIndex(0).AtMapKey("interval_ms"), knownvalue.Int64Exact(30000)),
					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions").AtSliceIndex(0).AtMapKey("metrics"), knownvalue.StringExact("org.apache.kafka.consumer.")),
					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions").AtSliceIndex(0).AtMapKey("match"), knownvalue.Null()),

					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions").AtSliceIndex(1).AtMapKey("name"), knownvalue.StringExact("producers")),
					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions").AtSliceIndex(1).AtMapKey("interval_ms"), knownvalue.Int64Exact(60000)),
					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions").AtSliceIndex(1).AtMapKey("metrics"), knownvalue.StringExact("org.apache.kafka.producer.")),
					statecheck.ExpectKnownValue(cmsListAddr, tfjsonpath.New("subscriptions").AtSliceIndex(1).AtMapKey("match"), knownvalue.StringExact("client_id=^app-.*")),
				},
			},
		},
	})
}
