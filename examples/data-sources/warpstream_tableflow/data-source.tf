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

data "warpstream_tableflow" "example_tableflow" {
  name = "vcn_tf_example_tableflow"
}

