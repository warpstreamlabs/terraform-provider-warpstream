data "warpstream_account" "current" {}

output "warpstream_api_key" {
  value     = data.warpstream_account.current.api_key
  sensitive = true
}
