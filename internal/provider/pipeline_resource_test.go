package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestBentoPipelineResourceDeletePlan(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create Pipeline
			{
				Config: testBentoPipeline(vcNameSuffix),
				Check:  testPipelineCheck(bentoPipelineType),
			},
			// Pre delete pipeline and try planning
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					vcs, err := client.GetVirtualClusters()
					require.NoError(t, err)

					var virtualCluster *api.VirtualCluster
					for _, vc := range vcs {
						if vc.Name == fmt.Sprintf("vcn_test_acc_%s", vcNameSuffix) {
							virtualCluster = &vc
							break
						}
					}
					require.NotNil(t, virtualCluster)

					pipelineListResp, err := client.ListPipelines(context.TODO(), api.HTTPListPipelinesRequest{
						VirtualClusterID: virtualCluster.ID,
					})
					require.NoError(t, err)

					_, err = client.DeletePipeline(context.TODO(), api.HTTPDeletePipelineRequest{
						VirtualClusterID: virtualCluster.ID,
						PipelineID:       pipelineListResp.Pipelines[0].ID,
					})
					require.NoError(t, err)

				},
				Config:             testBentoPipeline(vcNameSuffix),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			// Create pipeline
			{
				Config: testBentoPipeline(vcNameSuffix),
				Check:  testPipelineCheck(bentoPipelineType),
			},
			// Delete virtual cluster and try planning
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					vcs, err := client.GetVirtualClusters()
					require.NoError(t, err)

					var virtualCluster *api.VirtualCluster
					for _, vc := range vcs {
						if vc.Name == fmt.Sprintf("vcn_test_acc_%s", vcNameSuffix) {
							virtualCluster = &vc
							break
						}
					}
					require.NotNil(t, virtualCluster)

					err = client.DeleteVirtualCluster(virtualCluster.ID, virtualCluster.Name)
					require.NoError(t, err)
				},
				Config:             testBentoPipeline(vcNameSuffix),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestBentoPipelineResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testBentoPipeline(acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)),
				Check:  testPipelineCheck(bentoPipelineType),
			},
		},
	})
}

func testBentoPipeline(vcNameSuffix string) string {
	virtualClusterResource := fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
	name = "vcn_test_acc_%s"
}`, vcNameSuffix)
	return providerConfig + virtualClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  configuration_yaml = <<EOT
  input:
    kafka_franz:
        seed_brokers: ["localhost:9092"]
        topics: ["test_topic"]
        consumer_group: "test_topic_cap"

    processors:
        - mapping: "root = content().capitalize()"

  output:
      kafka_franz:
          seed_brokers: ["localhost:9092"]
          topic: "test_topic_capitalized"
  EOT
}`
}

func TestOrbitPipelineResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testOrbitPipeline(),
				Check:  testPipelineCheck(orbitPipelineType),
			},
		},
	})
}

func testOrbitPipeline() string {
	virtualClusterResource := fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
	name = "vcn_test_acc_kobe_%s"
}`, acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))
	return providerConfig + virtualClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  type				 = "orbit"
  configuration_yaml = <<EOT
  source_bootstrap_brokers:
    - hostname: localhost
      port: 9092
  
  source_cluster_credentials:
    sasl_mechanism: plain
    use_tls: false

  topic_mappings:
    - source_regex: topic.*
      destination_prefix: ""

  cluster_config:
    copy_source_cluster_configuration: false

  consumer_groups:
    copy_offsets_enabled: true             

  warpstream:
    cluster_fetch_concurrency: 2
  EOT
}`
}

func testPipelineCheck(
	pipelineType string,
) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_pipeline.test_pipeline", "id"),
		resource.TestCheckResourceAttrSet("warpstream_pipeline.test_pipeline", "configuration_id"),
		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "state", "running"),
		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "type", pipelineType),
	)
}
