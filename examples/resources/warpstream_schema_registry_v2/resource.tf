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

# Creates no agent key, like the v1 resource. Add one with warpstream_agent_key_v2.
resource "warpstream_schema_registry_v2" "example" {
  name = "vcn_sr_example"
  tier = "dev"
}

# Also creates an agent key (v2 key format) and keeps its raw secret in state.
resource "warpstream_schema_registry_v2" "example_with_key" {
  name             = "vcn_sr_example_with_key"
  tier             = "dev"
  create_agent_key = true
}

output "agent_key" {
  value     = warpstream_schema_registry_v2.example_with_key.agent_key.key
  sensitive = true
}
