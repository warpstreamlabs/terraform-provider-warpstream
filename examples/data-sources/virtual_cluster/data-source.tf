data "warpstream_virtual_cluster" "default" {
  id = "vci_532bcfb0_1948_4afb_9f6a_69175ca31f84"
}

output "vc_default_name" {
  value = data.warpstream_virtual_cluster.default.name
}
