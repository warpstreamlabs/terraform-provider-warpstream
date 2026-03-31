package tests

import (
	"fmt"
	"regexp"
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
					client, err := api.NewClientDefault()
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

					pipelineListResp, err := client.ListPipelines(t.Context(), api.HTTPListPipelinesRequest{
						VirtualClusterID: virtualCluster.ID,
					})
					require.NoError(t, err)

					_, err = client.DeletePipeline(t.Context(), api.HTTPDeletePipelineRequest{
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
					client, err := api.NewClientDefault()
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

func TestBentoPipelineResourceInvalidYamlUpdate(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create Pipeline with valid config
			{
				Config: testBentoPipeline(vcNameSuffix),
				Check:  testPipelineCheck(resources.BentoPipelineType),
			},
			// Try to update with invalid YAML - should error without deleting pipeline
			{
				Config:      testBentoPipelineInvalidYaml(vcNameSuffix),
				ExpectError: regexp.MustCompile(".*"),
			},
			// Verify pipeline still exists with original config
			{
				Config:             testBentoPipeline(vcNameSuffix),
				Check:              testPipelineCheck(resources.BentoPipelineType),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestBentoPipelineResourceValidYamlUpdate(t *testing.T) {
	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create Pipeline with initial config
			{
				Config: testBentoPipeline(vcNameSuffix),
				Check:  testPipelineCheck(resources.BentoPipelineType),
			},
			// Update with valid modified YAML - should update in place, not recreate
			{
				Config: testBentoPipelineUpdated(vcNameSuffix),
				Check:  testPipelineCheck(resources.BentoPipelineType),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_pipeline.test_pipeline", plancheck.ResourceActionUpdate),
					},
				},
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

func testBentoPipelineInvalidYaml(vcNameSuffix string) string {
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
  input:
    kafka_franz:
        seed_brokers: ["localhost:9092"]
        topics: ["test_topic"]
        consumer_group: "test_topic_cap"
        invalid_field_that_should_cause_error: true

    processors:
        - mapping: "root = content().capitalize()"

  output:
      this_is_invalid_output_type:
          seed_brokers: ["localhost:9092"]
          topic: "test_topic_capitalized"
  EOT
}`
}

func testBentoPipelineUpdated(vcNameSuffix string) string {
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
  input:
    kafka_franz:
        seed_brokers: ["localhost:9092"]
        topics: ["test_topic_updated"]
        consumer_group: "test_topic_cap_updated"

    processors:
        - mapping: "root = content().uppercase()"

  output:
      kafka_franz:
          seed_brokers: ["localhost:9092"]
          topic: "test_topic_uppercased"
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

func TestTableflowPipelineResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTableflowPipeline(),
				Check:  testPipelineCheck(resources.TableflowPipelineType),
			},
		},
	})
}

func testTableflowPipeline() string {
	tableflowClusterResource := fmt.Sprintf(`
resource "warpstream_tableflow_cluster" "test" {
  name = "vcn_dl_test_%s"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}`, acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))
	return providerConfig + tableflowClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_tableflow_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  type               = "tableflow"
  configuration_yaml = <<EOT
source_clusters:
  - name: kafka_cluster_1
    bootstrap_brokers:
      - hostname: localhost
        port: 9092
tables:
  - source_cluster_name: kafka_cluster_1
    source_topic: logs
    source_format: json
    schema_mode: inline
    schema:
      fields:
        - { name: environment, type: string, id: 1}
        - { name: service, type: string, id: 2}
        - { name: status, type: string, id: 3}
        - { name: message, type: string, id: 4}
destination_bucket_url: s3://test-tableflow-bucket?region=us-east-1
EOT
}`
}

func TestTableflowPipelineResourceWithInputs(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTableflowPipelineWithInputs(),
				Check:  testPipelineCheck(resources.TableflowPipelineType),
			},
			{
				Config: testTableflowPipelineWithInputs(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestTableflowPipelineResourceUpdateInputs(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with initial configuration_inputs
			{
				Config: testTableflowPipelineWithInputs(),
				Check:  testPipelineCheck(resources.TableflowPipelineType),
			},
			// Update with modified configuration_inputs (add a table part)
			{
				Config: testTableflowPipelineWithInputsUpdated(),
				Check:  testPipelineCheck(resources.TableflowPipelineType),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_pipeline.test_pipeline", plancheck.ResourceActionUpdate),
					},
				},
			},
			// Re-apply same config to verify state has settled
			{
				Config: testTableflowPipelineWithInputsUpdated(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestTableflowPipelineResourceInputsTreeStructure(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTableflowPipelineWithInputsTree(),
				Check:  testPipelineCheck(resources.TableflowPipelineType),
			},
			{
				Config: testTableflowPipelineWithInputsTree(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestTableflowPipelineResourceInputsValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// configuration_inputs on a non-tableflow pipeline should fail validation
			{
				Config:      testBentoPipelineWithInputs(),
				ExpectError: regexp.MustCompile("configuration_inputs is only supported for tableflow"),
			},
			// Both configuration_yaml and configuration_inputs should fail validation
			{
				Config:      testTableflowPipelineBothConfigAttrs(),
				ExpectError: regexp.MustCompile("Cannot specify both"),
			},
		},
	})
}

var (
	tableflowInputsClusterSuffix     = acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	tableflowInputsTreeClusterSuffix = acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
)

func testTableflowPipelineWithInputs() string {
	tableflowClusterResource := fmt.Sprintf(`
resource "warpstream_tableflow_cluster" "test" {
  name = "vcn_dl_test_%s"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}`, tableflowInputsClusterSuffix)
	return providerConfig + tableflowClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_tableflow_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  type               = "tableflow"
  configuration_inputs = {
    "globals" = <<-YAML
      source_clusters:
        - name: kafka_cluster_1
          bootstrap_brokers:
            - hostname: localhost
              port: 9092
      destination_bucket_url: s3://test-tableflow-bucket?region=us-east-1
    YAML
    "tables/logs" = <<-YAML
      tables:
        - source_cluster_name: kafka_cluster_1
          source_topic: logs
          source_format: json
          schema_mode: inline
          schema:
            fields:
              - { name: environment, type: string, id: 1 }
              - { name: service, type: string, id: 2 }
              - { name: status, type: string, id: 3 }
              - { name: message, type: string, id: 4 }
    YAML
  }
}`
}

func testTableflowPipelineWithInputsUpdated() string {
	tableflowClusterResource := fmt.Sprintf(`
resource "warpstream_tableflow_cluster" "test" {
  name = "vcn_dl_test_%s"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}`, tableflowInputsClusterSuffix)
	return providerConfig + tableflowClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_tableflow_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  type               = "tableflow"
  configuration_inputs = {
    "globals" = <<-YAML
      source_clusters:
        - name: kafka_cluster_1
          bootstrap_brokers:
            - hostname: localhost
              port: 9092
      destination_bucket_url: s3://test-tableflow-bucket?region=us-east-1
    YAML
    "tables/logs" = <<-YAML
      tables:
        - source_cluster_name: kafka_cluster_1
          source_topic: logs
          source_format: json
          schema_mode: inline
          schema:
            fields:
              - { name: environment, type: string, id: 1 }
              - { name: service, type: string, id: 2 }
              - { name: status, type: string, id: 3 }
              - { name: message, type: string, id: 4 }
    YAML
    "tables/events" = <<-YAML
      tables:
        - source_cluster_name: kafka_cluster_1
          source_topic: events
          source_format: json
          schema_mode: inline
          schema:
            fields:
              - { name: event_id, type: string, id: 1 }
              - { name: timestamp, type: long, id: 2 }
    YAML
  }
}`
}

func testTableflowPipelineWithInputsTree() string {
	tableflowClusterResource := fmt.Sprintf(`
resource "warpstream_tableflow_cluster" "test" {
  name = "vcn_dl_test_%s"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}`, tableflowInputsTreeClusterSuffix)
	return providerConfig + tableflowClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_tableflow_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  type               = "tableflow"
  configuration_inputs = {
    "globals" = <<-YAML
      source_clusters:
        - name: kafka_cluster_1
          bootstrap_brokers:
            - hostname: localhost
              port: 9092
      destination_bucket_url: s3://test-tableflow-bucket?region=us-east-1
    YAML
    "analytics/user_events" = <<-YAML
      tables:
        - source_cluster_name: kafka_cluster_1
          source_topic: user_events
          source_format: json
          schema_mode: inline
          schema:
            fields:
              - { name: user_id, type: string, id: 1 }
              - { name: action, type: string, id: 2 }
    YAML
    "logging/app_logs" = <<-YAML
      tables:
        - source_cluster_name: kafka_cluster_1
          source_topic: app_logs
          source_format: json
          schema_mode: inline
          schema:
            fields:
              - { name: level, type: string, id: 1 }
              - { name: message, type: string, id: 2 }
    YAML
  }
}`
}

func testBentoPipelineWithInputs() string {
	virtualClusterResource := fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
	name = "vcn_test_acc_%s"
    tier = "dev"
}`, acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))
	return providerConfig + virtualClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  type               = "bento"
  configuration_inputs = {
    "default" = <<-YAML
      input:
        kafka_franz:
          seed_brokers: ["localhost:9092"]
          topics: ["test_topic"]
    YAML
  }
}`
}

func testTableflowPipelineBothConfigAttrs() string {
	tableflowClusterResource := fmt.Sprintf(`
resource "warpstream_tableflow_cluster" "test" {
  name = "vcn_dl_test_%s"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}`, acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum))
	return providerConfig + tableflowClusterResource + `
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_tableflow_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  type               = "tableflow"
  configuration_yaml = "source_clusters: []"
  configuration_inputs = {
    "globals" = "source_clusters: []"
  }
}`
}

func TestPipelineResourceImport(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testOrbitPipeline(),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "warpstream_pipeline.test_pipeline",
			},
		},
	})
}

func TestPipelineResourceConfigurationYAMLFromVariable(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testPipelineConfigYAMLFromVariable(),
				Check:              testPipelineCheck(resources.TableflowPipelineType),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testPipelineConfigYAMLFromVariable() string {
	return providerConfig + `
variable "configuration_yaml" {
  type    = string
  default = <<-EOT
    input: {}
    output: {}
  EOT
}
resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = "vci_1234567890"
  name               = "test_pipeline"
  state              = "paused"
  configuration_yaml = var.configuration_yaml
}`
}
