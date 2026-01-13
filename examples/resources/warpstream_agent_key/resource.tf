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

resource "warpstream_virtual_cluster" "tf_example_agent_keys" {
  name = "vcn_tf_example_agent_keys"
  tier = "dev"
}

resource "warpstream_agent_key" "example_agent_key" {
  virtual_cluster_id = warpstream_virtual_cluster.tf_example_agent_keys.id
  name               = "akn_example_agent_key"
  read_only          = false
}

# This is a read-only agent key. It cannot be used to deploy Agents, but it can
# used to hit virtual-cluster specific read-only APIs, like the hosted Prometheus
# endpoint.
resource "warpstream_agent_key" "example_agent_key_read_only" {
  virtual_cluster_id = warpstream_virtual_cluster.tf_example_agent_keys.id
  name               = "akn_example_agent_key"
  read_only          = true
}
