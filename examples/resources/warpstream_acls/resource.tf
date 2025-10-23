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

resource "warpstream_virtual_cluster" "tf_example_acls" {
  name = "vcn_tf_example_acls"
  tier = "dev"
}

resource "warpstream_acl" "read_topic" {
  virtual_cluster_id = warpstream_virtual_cluster.tf_example_acls.id
  host               = "*"
  principal          = "User:test-user"
  operation          = "WRITE"
  permission_type    = "ALLOW"
  resource_type      = "TOPIC"
  resource_name      = "test-topic"
  pattern_type       = "LITERAL"
}
