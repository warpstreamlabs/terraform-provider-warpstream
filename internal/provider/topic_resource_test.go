package provider

import (
	"fmt"
	"os"
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
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTopicAndClusterResource(acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)),
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
    name = "retention.ms"
	value = "604800000"
  }
}
				`, acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)),
				Check: resource.ComposeAggregateTestCheckFunc(
					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("retention.ms")),
					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config").AtSliceIndex(0).AtMapKey("value"), knownvalue.StringExact("604800000")),
				},
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
