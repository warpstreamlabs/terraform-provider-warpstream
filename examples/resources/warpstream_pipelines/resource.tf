terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

provider "warpstream" {
  token    = "aks_xxx"
}

resource "warpstream_virtual_cluster" "tf_example_pipelines" {
  name = "vcn_tf_example_pipelines"
}

resource "warpstream_pipeline" "example_pipeline" {
  virtual_cluster_id             = warpstream_virtual_cluster.tf_example_pipelines.id
  name                           = "example_pipeline"
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
            topics: ["test_topic_2"]
            consumer_group: "test_topic_cg"

        processors:
            - mapping: "root = content().capitalize()"

      output:
          kafka_franz:
              seed_brokers: ["localhost:9092"]
              topic: "test_topic_capitalized_2"
      EOT
    },
  ]
}
