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
	virtual_cluster_id             = warpstream_virtual_cluster.test_pipeline.id
	name                           = "test_pipeline"
	state                          = "running"
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

func testPipelineCheck() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_pipeline.test_pipeline", "id"),
		resource.TestCheckResourceAttrSet("warpstream_pipeline.test_pipeline", "type"),
		resource.TestCheckResourceAttrSet("warpstream_pipeline.test_pipeline", "configuration_id"),

		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "name", "test_pipeline"),
		resource.TestCheckResourceAttr("warpstream_pipeline.test_pipeline", "state", "running"),
	)
}
