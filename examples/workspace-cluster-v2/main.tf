terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

# Your WarpStream account key, this can be found by clicking "Default workspace" in the top left of the
# WarpStream Console then clicking "Manage".
variable "warpstream_account_api_key" {
  description = "Your WarpStream Account API Key"
  sensitive   = true
}

# Create a warpstream provider with the account key
provider "warpstream" {
  token = var.warpstream_account_api_key
  alias = "account"
}

# Create a workspace
resource "warpstream_workspace_v2" "foo" {
  provider = warpstream.account # Using the account provider

  name = "foo"
}

# Create an application key for the workspace
resource "warpstream_application_key_v2" "workspace_foo" {
  provider = warpstream.account # Using the account provider

  name         = "akn_example_application_key"
  workspace_id = warpstream_workspace_v2.foo.id
}

# Create a user role that has admin access to the newly created workspace
resource "warpstream_user_role" "example_user_role" {
  provider = warpstream.account # Using the account provider

  name = "workspace-foo-admin"

  access_grants = [
    {
      workspace_id = warpstream_workspace_v2.foo.id
      grant_type   = "admin"
    },
  ]
}

# Create a provider for the workspace using the workspace's application key
provider "warpstream" {
  token = warpstream_application_key_v2.workspace_foo.key
  alias = "workspace-foo"
}

# Create a cluster
resource "warpstream_virtual_cluster_v2" "example" {
  provider = warpstream.workspace-foo # Using the workspace specific provider

  name = "vcn_example"
  type = "byoc"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}

resource "warpstream_agent_key_v2" "example" {
  provider = warpstream.workspace-foo # Using the workspace specific provider

  name               = "akn_example"
  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id
}

# The agent key authenticates the WarpStream Agents with the WarpStream control
# plane. This is what you'll use in your Agent helm chart. Read it with
# `terraform output -raw agent_key`.
output "agent_key" {
  value     = warpstream_agent_key_v2.example.key
  sensitive = true
}
