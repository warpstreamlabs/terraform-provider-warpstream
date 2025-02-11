package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// TODO: this currently doesn't work, the describe topic api returns a 500 instead of 404
// func TestAccTopicResourceNotExists(t *testing.T) {

// 	virtualClusterName := fmt.Sprintf("vcn_%s", acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))

// 	resource.Test(t, resource.TestCase{
// 		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
// 		Steps: []resource.TestStep{
// 			{
// 				Config: providerConfig + fmt.Sprintf(`
// resource "warpstream_virtual_cluster" "default" {
// 	name = "%s"
// }

// resource "warpstream_topic" "topic" {
//   topic_name         = "test"
//   partition_count    = 1
//   virtual_cluster_id = warpstream_virtual_cluster.default.id

// }
// 				`, virtualClusterName),
// 				Check: resource.ComposeAggregateTestCheckFunc(
// 					utils.TestCheckResourceAttrStartsWith("warpstream_topic.topic", "virtual_cluster_id", "vci_"),
// 				),
// 				ConfigStateChecks: []statecheck.StateCheck{
// 					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("topic_name"), knownvalue.StringExact("test")),
// 					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("partition_count"), knownvalue.Int64Exact(1)),
// 					statecheck.ExpectKnownValue("warpstream_topic.topic", tfjsonpath.New("config"), knownvalue.ListSizeExact(0)),
// 				},
// 			},
// 			{
// 				PreConfig: func() {
// 					token := os.Getenv("WARPSTREAM_API_KEY")
// 					client, _ := api.NewClient("", &token)
// 					// TODO: error

// 					vcs, _ := client.GetVirtualClusters()
// 					// TODO: err

// 					var virtualCluster *api.VirtualCluster
// 					for _, vc := range vcs {
// 						fmt.Println(vc.Name)
// 						if vc.Name == virtualClusterName {
// 							fmt.Println("found vc")
// 							virtualCluster = &vc
// 							break
// 						}
// 					}

// 					if virtualCluster == nil {
// 						panic("nil virtual cluster")
// 					}

// 					_ = client.DeleteTopic(virtualCluster.ID, "test")
// 					// TODO: err
// 				},
// 				RefreshState: true,
// 			},
// 		},
// 	})
// }

func TestAccTopicResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "default" {
	name = "vcn_%s"
}

resource "warpstream_topic" "topic" {
  topic_name         = "test"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.default.id

}
				`, acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)),
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
