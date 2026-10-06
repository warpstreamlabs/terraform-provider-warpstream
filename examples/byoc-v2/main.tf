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

# BYOC cluster created together with its agent key.
resource "warpstream_virtual_cluster_v2" "example" {
  name = "vcn_example"
  type = "byoc"
  tier = "dev"
  cloud = {
    # This is the cloud provider and region of the WarpStream control plane,
    # *not* the region where the WarpStream Agents are deployed.
    provider = "aws"
    region   = "us-east-1"
  }
}

resource "warpstream_topic" "topic" {
  topic_name         = "logs"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id
}

# These are client credentials to authenticate with the WarpStream Agents if
# authentication is enabled.
resource "warpstream_virtual_cluster_credentials" "creds" {
  name = "ccn_example"

  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id
}

# The agent key authenticates the WarpStream Agents with the WarpStream control
# plane. This is what you'll use in your Agent helm chart. Read it with
# `terraform output -raw agent_key`.
output "agent_key" {
  value     = warpstream_virtual_cluster_v2.example.agent_key.key
  sensitive = true
}
