terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

provider "warpstream" {
  # Use a generic WarpStream API key here, not a cluster-specific Agent key.
  token = "YOUR_API_KEY"
}

resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
  type = "byoc"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}

resource "warpstream_pipeline" "example_bento_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.test.id
  name               = "example_bento_pipeline"
  state              = "running"
  configuration_yaml = <<EOT
  input:
    kafka_franz:
        seed_brokers: ["localhost:9092"]
        topics: ["test_topics"]
        consumer_group: "test_topic"

    processors:
        - mapping: "root = content().capitalize()"

  output:
      kafka_franz:
          seed_brokers: ["localhost:9092"]
          topic: "test_topic_capitalized"
  EOT
}
