data "warpstream_virtual_cluster" "default" {
  id = "vci_XXXXXXXXXX"
}

output "vc_default_name" {
  value = data.warpstream_virtual_cluster.default.name
}
