# Workspace-wide application key, in the provider key's workspace.
resource "warpstream_application_key_v2" "example" {
  name = "akn_example"
}

# Application key scoped to one cluster's topics.
resource "warpstream_virtual_cluster_v2" "example" {
  name = "vcn_example"
  tier = "dev"
}

resource "warpstream_application_key_v2" "topics" {
  name               = "akn_example_topics"
  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id
  resource_kind      = "virtual_cluster_topics"
}
