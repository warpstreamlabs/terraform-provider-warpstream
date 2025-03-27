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

resource "warpstream_schema_registry" "example_schema_registry" {
  name = "vcn_sr_example_schema_registry"
}
