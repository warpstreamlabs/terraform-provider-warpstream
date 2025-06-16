package tests

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/resources"
)

func TestBentoPipelineResourceDeletePlan(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create Pipeline
			{
				Config: testBentoPipeline(vcNameSuffix),
				Check:  testPipelineCheck(resources.BentoPipelineType),
			},
			// Pre delete pipeline and try planning
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					vcs, err := client.GetVirtualClusters()
					require.NoError(t, err)

					var virtualCluster api.VirtualCluster
					for _, vc := range vcs {
						if vc.Name == fmt.Sprintf("vcn_test_acc_%s", vcNameSuffix) {
							virtualCluster = vc
							break
						}
					}
					require.NotEmpty(t, virtualCluster.ID)

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
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_pipeline.test_pipeline", plancheck.ResourceActionCreate),
					},
				},
			},
			// Create pipeline
			{
				Config: testBentoPipeline(vcNameSuffix),
				Check:  testPipelineCheck(resources.BentoPipelineType),
			},
			// Delete virtual cluster and try planning
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					vcs, err := client.GetVirtualClusters()
					require.NoError(t, err)

					var virtualCluster api.VirtualCluster
					for _, vc := range vcs {
						if vc.Name == fmt.Sprintf("vcn_test_acc_%s", vcNameSuffix) {
							virtualCluster = vc
							break
						}
					}
					require.NotEmpty(t, virtualCluster.ID)

					err = client.DeleteVirtualCluster(virtualCluster.ID, virtualCluster.Name)
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_virtual_cluster.test", plancheck.ResourceActionCreate),
						plancheck.ExpectResourceAction("warpstream_pipeline.test_pipeline", plancheck.ResourceActionCreate),
					},
				},
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
				Check:  testPipelineCheck(resources.BentoPipelineType),
			},
		},
	})
}

func testBentoPipeline(vcNameSuffix string) string {
	virtualClusterResource := fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
	name = "vcn_test_acc_%s"
    tier = "dev"
}`, vcNameSuffix)
	return providerConfig + virtualClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  configuration_yaml = <<EOT
  error_handling:
    strategy: reject
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
				Check:  testPipelineCheck(resources.OrbitPipelineType),
			},
		},
	})
}

func testOrbitPipeline() string {
	virtualClusterResource := fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
	name = "vcn_test_acc_kobe_%s"
    tier = "dev"
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

func TestSchemaMigratorPipelineResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testSchemaMigratorPipeline(),
				Check:  testPipelineCheck(resources.SchemaLinkingPipelineType),
			},
		},
	})
}

func testSchemaMigratorPipeline() string {
	virtualClusterResource := fmt.Sprintf(`
resource "warpstream_schema_registry" "test" {
  name = "vcn_sr_test_%s"
}`, acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))
	return providerConfig + virtualClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_schema_registry.test.id
  name               = "test_pipeline"
  state              = "running"
  type				 = "schema_linking"
  configuration_yaml = <<EOT
source_schema_registry:
  hostname: "localhost"
  port: 8087
sync_every_seconds: 300
context_type: "DEFAULT"
EOT
}`
}
