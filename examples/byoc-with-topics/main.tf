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
  virtual_cluster_id = warpstream_virtual_cluster.test.id

  config {
    name  = "retention.ms"
    value = "604800000"
  }
}

resource "warpstream_virtual_cluster_credentials" "creds" {
  name = "ccn_test"

  virtual_cluster_id = warpstream_virtual_cluster.test.id
}
