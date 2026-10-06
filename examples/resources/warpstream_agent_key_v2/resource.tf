resource "warpstream_virtual_cluster_v2" "example" {
  name = "vcn_example"
  tier = "dev"
}

resource "warpstream_agent_key_v2" "example" {
  name               = "akn_example_read_only"
  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id
  read_only          = true
}
