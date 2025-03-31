resource "warpstream_virtual_cluster" "xxx" {
  name = "vcn_xxx"
  tier = "dev"
}

resource "warpstream_virtual_cluster_credentials" "test" {
  name               = "ccn_test"
  agent_pool         = warpstream_virtual_cluster.xxx.agent_pool_id
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
