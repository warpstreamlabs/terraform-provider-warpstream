data "warpstream_application_keys" "all" {
}

output "first_application_key_secret" {
  value     = data.warpstream_application_keys.all.application_keys.0.key
  sensitive = true
}
