resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
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
    default_num_partitions = 1
    auto_create_topic      = true
    enable_acls            = true
  }
}
