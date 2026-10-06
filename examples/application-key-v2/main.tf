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

# Read-only application key for the provider key's workspace, for example for
# a monitoring tool.
resource "warpstream_application_key_v2" "read_only" {
  name      = "akn_example_read_only"
  read_only = true
}

resource "warpstream_virtual_cluster_v2" "example" {
  name = "vcn_example"
  type = "byoc"
  tier = "dev"
}

# Application key that can only manage the topics of this cluster, for example
# for a CI pipeline that creates topics.
resource "warpstream_application_key_v2" "topics" {
  name               = "akn_example_topics"
  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id
  resource_kind      = "virtual_cluster_topics"
}

# Read them with `terraform output -raw read_only_application_key` and
# `terraform output -raw topics_application_key`.
output "read_only_application_key" {
  value     = warpstream_application_key_v2.read_only.key
  sensitive = true
}

output "topics_application_key" {
  value     = warpstream_application_key_v2.topics.key
  sensitive = true
}
