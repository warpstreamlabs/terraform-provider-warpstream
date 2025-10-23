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

resource "warpstream_virtual_cluster" "acl_example" {
  name = "vcn_example_acls"
  tier = "dev"
  configuration = {
    enable_acls = true
  }
}

resource "warpstream_acl" "read_topic" {
  virtual_cluster_id = warpstream_virtual_cluster.acl_example.id
  host               = "*"
  principal          = "User:test-user"
  operation          = "WRITE"
  permission_type    = "ALLOW"
  resource_type      = "TOPIC"
  resource_name      = "test-topic"
  pattern_type       = "LITERAL"
}
