resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
  tier = "dev"
}

resource "warpstream_client_metrics_subscription" "producers" {
  virtual_cluster_id = warpstream_virtual_cluster.test.id
  name               = "producers"

  interval_ms = 60000
  metrics     = "org.apache.kafka.producer."
  match       = "client_id=^app-.*"
}
