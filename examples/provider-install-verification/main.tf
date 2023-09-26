terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

provider "warpstream" {
  ## Uncomment or export environment variable WARPSTREAM_API_KEY
  #token = "your-warpstream-api-key"
}

data "warpstream_virtual_clusters" "all" {
}
