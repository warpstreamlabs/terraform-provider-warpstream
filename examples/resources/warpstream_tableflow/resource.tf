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

resource "warpstream_tableflow" "dev_cluster" {
  name = "vcn_dl_dev_tableflow"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}

resource "warpstream_tableflow" "fundamentals_cluster" {
  name = "vcn_dl_fundamentals_tableflow"
  tier = "fundamentals"
  cloud = {
    provider = "aws"
    region   = "us-east-2"
  }
}

resource "warpstream_tableflow" "enterprise_cluster" {
  name = "vcn_dl_pro_tableflow"
  tier = "enterprise"
  cloud = {
    provider = "gcp"
    region   = "us-central1"
  }
}
