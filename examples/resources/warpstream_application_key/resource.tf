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

resource "warpstream_virtual_cluster" "example" {
  name = "vcn_example"
  tier = "dev"
}

resource "warpstream_application_key" "example_cluster_scoped_application_key" {
  name               = "akn_example_cluster_scoped_application_key"
  virtual_cluster_id = warpstream_virtual_cluster.example.id
  resource_kind      = "virtual_cluster_topics"
}
