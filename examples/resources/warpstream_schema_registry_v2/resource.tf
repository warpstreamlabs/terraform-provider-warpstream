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

resource "warpstream_schema_registry_v2" "example" {
  name = "vcn_sr_example"
  tier = "dev"
}

output "agent_key" {
  value     = warpstream_schema_registry_v2.example.agent_key.key
  sensitive = true
}
