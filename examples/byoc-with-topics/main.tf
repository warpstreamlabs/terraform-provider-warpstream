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

resource "warpstream_topic" "topic" {
  topic_name         = "logs"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.test.id

  config {
    name  = "retention.ms"
    value = "604800000"
  }
}
