package tests

import (
	"fmt"
	"os"
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

func TestAccTopicResourceMultipleConfigs(t *testing.T) {
	var cluster = acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTopicAndClusterResource(cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(0)),
				},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
    tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id

  config {
    name = "cleanup.policy"
	value = "delete"
  }

  config {
    name = "retention.ms"
	value = "604800000"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("cleanup.policy")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("delete")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(1).AtMapKey("name"), knownvalue.StringExact("retention.ms")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(1).AtMapKey("value"), knownvalue.StringExact("604800000")),
				},
			},
			// No OP Change
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
    tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id

  config {
    name = "cleanup.policy"
	value = "delete"
  }

  config {
    name = "retention.ms"
	value = "604800000"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Change order of topic configuration
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
    tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id

  config {
    name = "retention.ms"
	value = "604800000"
  }

  config {
    name = "cleanup.policy"
	value = "delete"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Add topic config in the middle
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
    tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id

  config {
    name = "retention.ms"
	value = "604800000"
  }

  config {
    name = "warpstream.compression.type.fetch"
	value = "lz4"
  }

  config {
    name = "cleanup.policy"
	value = "delete"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(3)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("cleanup.policy")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("delete")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(1).AtMapKey("name"), knownvalue.StringExact("retention.ms")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(1).AtMapKey("value"), knownvalue.StringExact("604800000")),
					// With a set new items get added to the end so we are testing for #2 for the new config
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(2).AtMapKey("name"), knownvalue.StringExact("warpstream.compression.type.fetch")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(2).AtMapKey("value"), knownvalue.StringExact("lz4")),
				},
			},
			// Remove first config
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
    tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id

  config {
    name = "warpstream.compression.type.fetch"
	value = "lz4"
  }

  config {
    name = "cleanup.policy"
	value = "delete"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("cleanup.policy")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("delete")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(1).AtMapKey("name"), knownvalue.StringExact("warpstream.compression.type.fetch")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(1).AtMapKey("value"), knownvalue.StringExact("lz4")),
				},
			},
			// Add topic config to the end
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
    tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id

  config {
    name = "warpstream.compression.type.fetch"
	value = "lz4"
  }

  config {
    name = "cleanup.policy"
	value = "delete"
  }

  config {
    name = "retention.ms"
	value = "604800000"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					// The order of the slice is different then defined in terraform hcl
					// Sets are unordered so the ordering isn't guaranteed to match
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(3)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("cleanup.policy")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("delete")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(1).AtMapKey("name"), knownvalue.StringExact("retention.ms")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(1).AtMapKey("value"), knownvalue.StringExact("604800000")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(2).AtMapKey("name"), knownvalue.StringExact("warpstream.compression.type.fetch")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(2).AtMapKey("value"), knownvalue.StringExact("lz4")),
				},
			},
			// No OP Change again to make sure
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
    tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id

  config {
    name = "warpstream.compression.type.fetch"
	value = "lz4"
  }

  config {
    name = "cleanup.policy"
	value = "delete"
  }

  config {
    name = "retention.ms"
	value = "604800000"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccTopicResourceDeletePlan(t *testing.T) {
	virtualClusterRandString := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	virtualClusterName := fmt.Sprintf("vcn_%s", virtualClusterRandString)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create topic
			{
				Config: testAccTopicAndClusterResource(virtualClusterRandString),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(0)),
				},
			},
			// Pre delete topic and try planning
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					virtualCluster, err := client.FindVirtualCluster(virtualClusterName)
					require.NoError(t, err)

					err = client.DeleteTopic(virtualCluster.ID, "test")
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_topic.topic", plancheck.ResourceActionCreate),
					},
				},
			},
			// Create topic
			{
				Config: testAccTopicAndClusterResource(virtualClusterRandString),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(0)),
				},
			},
			// Delete virtual cluster and try planning
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					virtualCluster, err := client.FindVirtualCluster(virtualClusterName)
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
						plancheck.ExpectResourceAction("warpstream_topic.topic", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccTopicResource(t *testing.T) {
	var cluster = acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTopicAndClusterResource(cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(0)),
				},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
    tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id
  enable_deletion_protection = true

  config {
    name = "retention.ms"
	value = "604800000"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
					resource.TestCheckNoResourceAttr("warpstream_topic.topic", "config.1.name"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("retention.ms")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("604800000")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("enable_deletion_protection"), knownvalue.Bool(true)),
				},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
	tier = "dev"
}
				`, cluster),
				ExpectError: regexp.MustCompile("deletion protection enabled"),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
	tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id
  enable_deletion_protection = false

  config {
    name = "retention.ms"
	value = "604800000"
  }
}
				`, cluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
					resource.TestCheckNoResourceAttr("warpstream_topic.topic", "config.1.name"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("retention.ms")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("604800000")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("enable_deletion_protection"), knownvalue.Bool(false)),
				},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
	tier = "dev"
}
				`, cluster),
			},
		},
	})
}

func testAccTopicAndClusterResource(clusterName string) string {
	return providerConfig + fmt.Sprintf(`
	resource "warpstream_virtual_cluster" "default" {
		name = "vcn_%s"
        tier = "dev"
	}
	resource "warpstream_topic" "topic" {
	  topic_name         = "test"
	  partition_count    = 1
	  virtual_cluster_id = warpstream_virtual_cluster.default.id
	}`, clusterName)
}

func TestAccTopicImport(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTopicAndClusterResource(acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "warpstream_topic.topic",
			},
		},
		IsUnitTest: true,
	})
}
