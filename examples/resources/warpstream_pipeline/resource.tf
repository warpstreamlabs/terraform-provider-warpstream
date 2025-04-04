terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

provider "warpstream" {
  token = "aks_xxx"
}

resource "warpstream_virtual_cluster" "tf_example_pipelines" {
  name = "vcn_tf_example_pipelines"
  tier = "dev"
}

resource "warpstream_pipeline" "example_bento_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.tf_example_pipelines.id
  name               = "example_bento_pipeline"
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
}

resource "warpstream_pipeline" "example_orbit_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.tf_example_pipelines.id
  name               = "example_orbit_pipeline"
  state              = "running"
  type               = "orbit"
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
}

resource "warpstream_pipeline" "example_schema_linking_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.tf_example_pipelines.id
  name               = "example_schema_linking_pipeline"
  state              = "running"
  type               = "schema_linking"
  configuration_yaml = <<EOT
  source_schema_registry:
    hostname: localhost
    port: 8087
  sync_every_seconds: 300
  context_type: "DEFAULT"
  EOT
}
