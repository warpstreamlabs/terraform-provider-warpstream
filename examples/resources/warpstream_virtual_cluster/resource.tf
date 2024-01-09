resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
}

resource "warpstream_virtual_cluster" "test_with_acls" {
  name = "vcn_test_acls"
  configuration = {
    enable_acls = true
  }
}
