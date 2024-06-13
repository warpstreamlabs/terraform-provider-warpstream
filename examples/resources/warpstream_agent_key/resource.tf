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
}

resource "warpstream_agent_key" "example_agent_key" {
  virtual_cluster_id = warpstream_virtual_cluster.tf_example_agent_keys.id
  name               = "akn_example_agent_key"
}
