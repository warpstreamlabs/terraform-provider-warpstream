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

resource "warpstream_application_key" "example_application_key" {
  name = "akn_example_application_key"
}

resource "warpstream_application_key" "example_read_only_application_key" {
  name      = "akn_example_read_only_application_key"
  read_only = true
}
