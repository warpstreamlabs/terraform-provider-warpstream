resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
  tier = "dev"
}

resource "warpstream_topic" "topic" {
  topic_name         = "logs"
  partition_count    = 1
  virtual_cluster_id = warpstream_virtual_cluster.test.id

  config {
    name  = "retention.ms"
    value = "604800000"
  }
}
