resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
}

resource "warpstream_virtual_cluster" "test_serverless" {
  name = "vcn_test_serverless"
  type = "serverless"
}

resource "warpstream_virtual_cluster" "test_with_acls" {
  name = "vcn_test_acls"
  configuration = {
    enable_acls = true
  }
}

resource "warpstream_virtual_cluster" "test_configuration" {
  name = "vcn_test_configuration"
  configuration = {
    auto_create_topic        = true
    default_num_partitions   = 1
    default_retention_millis = 86400000
    enable_acls              = true
  }
}

resource "warpstream_virtual_cluster" "test_cloud_region" {
  name = "vcn_test_cloud_region"
  cloud = {
    provider = "aws"
    region   = "ap-southeast-1"
  }
}
