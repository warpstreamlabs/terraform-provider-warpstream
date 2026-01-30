resource "warpstream_virtual_cluster" "xxx" {
  name = "vcn_xxx"
  tier = "dev"
}

resource "warpstream_virtual_cluster_credentials" "test" {
  name               = "ccn_test"
  virtual_cluster_id = warpstream_virtual_cluster.xxx.id
}

output "vcc_test_username" {
  value = resource.warpstream_virtual_cluster_credentials.test.username
}

# terraform output -raw vcc_test_password
output "vcc_test_password" {
  value     = resource.warpstream_virtual_cluster_credentials.test.password
  sensitive = true
}

# Takes an optional password field to assign a specific password to the new credentials instead of a random string.
# This is useful when migrating credentials from another WarpStream virtual cluster or from another data streaming platform.
resource "warpstream_virtual_cluster_credentials" "test-imported" {
  name               = "ccn_test_imported"
  password           = "S3cureP@ssw0rd!"
  virtual_cluster_id = warpstream_virtual_cluster.xxx.id
}
