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

# BYOC cluster with configuration.
resource "warpstream_virtual_cluster" "example" {
  name = "vcn_example"
  type = "byoc"
  tier = "dev"
  configuration = {
    auto_create_topic        = false
    default_num_partitions   = 1
    default_retention_millis = 86400000

    # Make it impossible to delete this cluster.
    enable_deletion_protection = true
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

resource "warpstream_topic" "topic" {
  topic_name         = "logs"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.example.id

  config {
    name  = "retention.ms"
    value = "604800000"
  }
}

# This is an agent key to authenticate the WarpStream Agents with the WarpStream
# control plane. This is what you'll use in your Agent helm chart.
resource "warpstream_agent_key" "example_agent_key" {
  virtual_cluster_id = warpstream_virtual_cluster.example.id
  name               = "akn_example_agent_key"
}

# These are client credentials to authenticate with the WarpStream Agents if
# authentication is enabled.
resource "warpstream_virtual_cluster_credentials" "creds" {
  name = "ccn_example"

  virtual_cluster_id = warpstream_virtual_cluster.example.id
}

