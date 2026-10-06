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
}

# An extra, read-only agent key. Read-only keys cannot be used to deploy
# Agents, only to call read-only APIs like the hosted Prometheus endpoint.
resource "warpstream_agent_key_v2" "read_only" {
  name               = "akn_example_read_only"
  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id
  read_only          = true
}

# Read it with `terraform output -raw read_only_agent_key`.
output "read_only_agent_key" {
  value     = warpstream_agent_key_v2.read_only.key
  sensitive = true
}
