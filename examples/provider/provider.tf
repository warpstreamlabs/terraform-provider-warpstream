terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

# Configuration-based authentication
provider "warpstream" {
  token = "YOUR_API_KEY"
}

