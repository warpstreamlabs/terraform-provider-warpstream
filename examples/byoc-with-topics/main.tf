terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
    kafka = {
      source = "Mongey/kafka"
    }
  }
}

provider "warpstream" {
  # Use a generic WarpStream API key here, not a cluster-specific Agent key.
  token = "YOUR_API_KEY"
}

# BYOC cluster with configuration.
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
  type = "byoc"
  configuration = {
    auto_create_topic        = false
    default_num_partitions   = 1
    default_retention_millis = 86400000
  }
  cloud = {
    # This is the cloud provider and region of the WarpStream control plane,
    # *not* the region where the WarpStream Agents are deployed. Agents can
    # be deployed anywhere and should connect to the nearest available
    # WarpStream control plane region.
    provider = "aws"
    region   = "us-east-1"
  }
}

# Create an Agent key that is dedicated to this terraform module for creating
# and configuring topics for this WarpStream cluster.
resource "warpstream_agent_key" "terraform_cluster_key" {
  virtual_cluster_id = warpstream_virtual_cluster.test.id
  name               = "akn_terraform_topics"
}

provider "kafka" {
  # WarpStream's serverless endpoint can be used to administer metadata
  # for BYOC clusters.
  bootstrap_servers = ["serverless.warpstream.com:9092"]
  tls_enabled       = true
  sasl_mechanism    = "plain"
  sasl_username     = "${warpstream_virtual_cluster.test.cloud.region}::${warpstream_virtual_cluster.test.id}"
  sasl_password     = warpstream_agent_key.terraform_cluster_key.key
}


resource "kafka_topic" "logs" {
  name       = "logs"
  partitions = 64
  # Required argument, but has no impact since data is always stored in object storage.
  replication_factor = 1

  config = {
    "retention.ms" = "86400000"
  }
}
