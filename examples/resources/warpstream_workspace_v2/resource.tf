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

# Creates no application key, like the v1 resource. Add one with warpstream_application_key_v2.
resource "warpstream_workspace_v2" "example" {
  name = "example-workspace"
}

# Also creates an application key (v2 key format) and keeps its raw secret in state.
resource "warpstream_workspace_v2" "example_with_key" {
  name                   = "example-workspace-with-key"
  create_application_key = true
}

output "application_key" {
  value     = warpstream_workspace_v2.example_with_key.application_key.key
  sensitive = true
}
