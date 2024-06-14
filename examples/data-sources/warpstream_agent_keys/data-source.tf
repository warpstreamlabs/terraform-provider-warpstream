data "warpstream_agent_keys" "all" {
}

output "first_agent_key_secret" {
  value     = data.warpstream_agent_keys.all.agent_keys.0.key
  sensitive = true
}
