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

data "warpstream_tableflow" "by_id" {
  id = "vci_dl_XXXXXXXXXX"
}

data "warpstream_tableflow" "by_name" {
  name = "vcn_dl_example_tableflow"
}

output "tableflow_tier" {
  value = data.warpstream_tableflow.by_name.tier
}

