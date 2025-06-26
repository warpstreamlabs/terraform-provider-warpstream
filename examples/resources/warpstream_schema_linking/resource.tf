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

resource "warpstream_schema_registry" "example_schema_registry" {
  name = "vcn_sr_example_schema_registry"
}

resource "warpstream_pipeline" "example_schema_linking_pipeline" {
  virtual_cluster_id = warpstream_schema_registry.example_schema_registry.id
  name               = "example_schema_linking_pipeline"
  state              = "running"
  type               = "schema_linking"
  configuration_yaml = <<EOT
  source_schema_registry:
    hostname: localhost
    port: 8087
  sync_every_seconds: 300
  context_type: "DEFAULT"
  EOT
}
