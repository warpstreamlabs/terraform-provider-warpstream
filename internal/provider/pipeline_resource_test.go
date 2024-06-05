package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestPipelineResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testPipeline(),
				Check:  testPipelineCheck(),
			},
		},
	})
}

func testPipeline() string {
	return providerConfig + `
data "warpstream_virtual_cluster" "default" {
	default = true
}

resource "warpstream_pipeline" "test_pipeline" {
	virtual_cluster_id             = data.warpstream_virtual_cluster.default.id
	name                           = "test_pipeline"
	state                          = "running"
	deployed_configuration_version = 1
	configurations = [
		{
		version            = 0
		configuration_yaml = <<EOT
		input:
			kafka_franz:
				seed_brokers: ["localhost:9092"]
				topics: ["test_topic"]
				consumer_group: "test_topic_cg"

			processors:
				- mapping: "root = content().capitalize()"

		output:
			kafka_franz:
				seed_brokers: ["localhost:9092"]
				topic: "test_topic_capitalized"
		EOT
		},
		{
		version            = 1
		configuration_yaml = <<EOT
		input:
			kafka_franz:
				seed_brokers: ["localhost:9092"]
				topics: ["test_topic"]
				consumer_group: "test_topic_cg"

			processors:
				- mapping: "root = content().capitalize()"

		output:
			kafka_franz:
				seed_brokers: ["localhost:9092"]
				topic: "test_topic_capitalized"
		EOT
		},
	]
}`
}

func testPipelineCheck() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_pipeline.test_pipeline", "id"),
		resource.TestCheckResourceAttrSet("warpstream_pipeline.test_pipeline", "type"),

		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "name", "test_pipeline"),
		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "state", "running"),
		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "deployed_configuration_version", "1"),

		// Add this check to validate the number of configurations.
		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "configurations.#", "2"),
		// Check individual configuration details.
		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "configurations.0.version", "0"),
		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "configurations.1.version", "1"),
	)
}
