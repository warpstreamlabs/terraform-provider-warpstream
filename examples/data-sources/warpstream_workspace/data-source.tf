data "warpstream_workspace" "example_workspace" {
  id = "wi_XXXXXXXXXX"
}

output "example_workspace_application_key_names" {
  value = [for application_key in data.warpstream_workspace.example_workspace.application_keys : application_key.name]
}
