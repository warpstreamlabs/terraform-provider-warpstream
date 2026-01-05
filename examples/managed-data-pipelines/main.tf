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

resource "warpstream_virtual_cluster" "example" {
  name = "vcn_example"
  type = "byoc"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}

# This is an agent key to authenticate the WarpStream Agents with the WarpStream
# control plane. This is what you'll use in your Agent helm chart.
resource "warpstream_agent_key" "example_agent_key" {
  virtual_cluster_id = warpstream_virtual_cluster.example.id
  name               = "akn_example_agent_key"
}

resource "warpstream_pipeline" "example_bento_pipeline" {
  virtual_cluster_id = warpstream_virtual_cluster.example.id
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
