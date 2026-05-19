data "warpstream_client_metrics_subscriptions" "all" {
  virtual_cluster_id = "vci_XXXXXXXXXX"
}

output "first_subscription_name" {
  value = data.warpstream_client_metrics_subscriptions.all.subscriptions.0.name
}

output "subscription_count" {
  value = length(data.warpstream_client_metrics_subscriptions.all.subscriptions)
}
