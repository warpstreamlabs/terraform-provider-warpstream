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

resource "warpstream_workspace_v2" "example" {
  name = "example-workspace"
}

output "application_key" {
  value     = warpstream_workspace_v2.example.application_key.key
  sensitive = true
}
