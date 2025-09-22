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

resource "warpstream_workspace" "example_workspace" {
  name = "example-workspace"
}


resource "warpstream_workspace" "example_imported_workspace" {
  name = "example-imported-workspace"
}
