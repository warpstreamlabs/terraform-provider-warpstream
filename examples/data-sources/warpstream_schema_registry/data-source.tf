data "warpstream_schema_registry" "by_id" {
  id = "vci_sr_XXXXXXXXXX"
}

data "warpstream_schema_registry" "by_name" {
  name = "vcn_sr_XXXXXXXXXX"
}

output "vc_sr_by_name_id" {
  value = data.warpstream_schema_registry.by_name.id
}
