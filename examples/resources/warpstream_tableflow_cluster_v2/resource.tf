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

resource "warpstream_tableflow_cluster_v2" "example" {
  name = "vcn_dl_example"
  tier = "dev"
}

output "agent_key" {
  value     = warpstream_tableflow_cluster_v2.example.agent_key.key
  sensitive = true
}
