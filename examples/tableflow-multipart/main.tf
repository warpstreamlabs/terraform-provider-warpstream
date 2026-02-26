terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

provider "warpstream" {
  # Use a generic WarpStream API key here, not a cluster-specific Agent key.
  token = "YOUR_API_KEY"
}

resource "warpstream_tableflow_cluster" "example" {
  name = "vcn_dl_example"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}

# Multi-part configuration: each part is a separate YAML file that gets merged
# server-side. Different teams can own different parts (e.g. analytics vs logging).
#
# The fileset() pattern automatically discovers all .yaml files in a directory
# tree and uses relative paths as part names, creating a tree hierarchy.
resource "warpstream_pipeline" "tableflow_pipeline" {
  virtual_cluster_id = warpstream_tableflow_cluster.example.id
  name               = "example_tableflow_pipeline"
  state              = "running"
  type               = "tableflow"

  configuration_inputs = {
    for f in fileset("${path.module}/config", "**/*.yaml") :
    trimsuffix(f, ".yaml") => file("${path.module}/config/${f}")
  }
}

# The above fileset() expression discovers config files and produces:
#
#   configuration_inputs = {
#     "globals"                  = <contents of config/globals.yaml>
#     "analytics/user_events"    = <contents of config/analytics/user_events.yaml>
#     "logging/app_logs"         = <contents of config/logging/app_logs.yaml>
#   }
#
# To add a new table, just create a new .yaml file in the appropriate directory
# and run `terraform apply`.
