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

resource "warpstream_schema_registry_v2" "example" {
  name = "vcn_sr_example"
  tier = "dev"
  cloud = {
    # This is the cloud provider and region of the WarpStream control plane,
    # *not* the region where the WarpStream Agents are deployed.
    provider = "aws"
    region   = "us-east-1"
  }
}

resource "warpstream_agent_key_v2" "example" {
  name               = "akn_example"
  virtual_cluster_id = warpstream_schema_registry_v2.example.id
}

# The agent key authenticates the WarpStream Schema Registry Agents with the
# WarpStream control plane. This is what you'll use in your Agent helm chart.
# Read it with `terraform output -raw agent_key`.
output "agent_key" {
  value     = warpstream_agent_key_v2.example.key
  sensitive = true
}

# The URL Schema Registry clients connect to.
output "schema_registry_url" {
  value = warpstream_schema_registry_v2.example.bootstrap_url
}
