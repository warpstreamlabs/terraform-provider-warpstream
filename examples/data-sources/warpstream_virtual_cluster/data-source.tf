data "warpstream_virtual_cluster" "default" {
  default = true
}

data "warpstream_virtual_cluster" "by_id" {
  id = "vci_XXXXXXXXXX"
}

data "warpstream_virtual_cluster" "by_name" {
  name = "vcn_XXXXXXXXXX"
}

output "vc_default_id" {
  value = data.warpstream_virtual_cluster.default.id
}
