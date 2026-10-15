---
page_title: "Using v2 keys"
subcategory: ""
description: |-
  How to create and manage agent and application keys in the v2 key format.
---

# Using v2 keys

WarpStream returns the raw secret of a key in the v2 key format only once, when the key is created.
It can't be read back afterwards, from the API or from the WarpStream Console.

The v2 resources create keys in this format and keep the raw secret in Terraform state.

## The Terraform state holds the only copy of the secret

Because the secret can't be read back, the Terraform state is the only place it is stored.
Treat the state as a secret: keep it in a remote backend that encrypts it at rest and restrict who can read it.

If the state is lost, the secret can't be recovered. Create a new key and delete the old one.

## v2 resources

| v2 resource | v1 resource |
|-------------|-------------|
| `warpstream_agent_key_v2` | `warpstream_agent_key` |
| `warpstream_application_key_v2` | `warpstream_application_key` |
| `warpstream_virtual_cluster_v2` | `warpstream_virtual_cluster` |
| `warpstream_schema_registry_v2` | `warpstream_schema_registry` |
| `warpstream_tableflow_cluster_v2` | `warpstream_tableflow_cluster` |
| `warpstream_workspace_v2` | `warpstream_workspace` |

## Getting a key for a cluster or workspace

Like the v1 resources, the v2 cluster and workspace resources create no key by default. There are two ways to get one.

### A separate key resource (recommended)

Create the key with `warpstream_agent_key_v2` for a cluster, or `warpstream_application_key_v2` for a workspace.
The key can be added to an existing cluster or workspace, rotated, or deleted at any time.

```terraform
resource "warpstream_virtual_cluster_v2" "example" {
  name = "vcn_example"
  tier = "dev"
}

resource "warpstream_agent_key_v2" "example" {
  name               = "akn_example"
  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id
}

# Pass this to the WarpStream Agents. Read it with `terraform output -raw agent_key`.
output "agent_key" {
  value     = warpstream_agent_key_v2.example.key
  sensitive = true
}
```

A workspace and its application key are created with an account key. The application key can then
authenticate a second provider that manages resources in that workspace:

```terraform
provider "warpstream" {
  token = var.warpstream_account_api_key
  alias = "account"
}

resource "warpstream_workspace_v2" "example" {
  provider = warpstream.account

  name = "example"
}

resource "warpstream_application_key_v2" "example" {
  provider = warpstream.account

  name         = "akn_example_application_key"
  workspace_id = warpstream_workspace_v2.example.id
}

provider "warpstream" {
  token = warpstream_application_key_v2.example.key
  alias = "example"
}

resource "warpstream_virtual_cluster_v2" "example" {
  provider = warpstream.example

  name = "vcn_example"
  tier = "dev"
}
```

### A key created with the cluster or workspace

Set `create_agent_key = true` on `warpstream_virtual_cluster_v2`, `warpstream_schema_registry_v2` or
`warpstream_tableflow_cluster_v2`, or `create_application_key = true` on `warpstream_workspace_v2`.
The resource then creates the key with the object and keeps it in its `agent_key` or `application_key` attribute.

```terraform
resource "warpstream_virtual_cluster_v2" "example" {
  name             = "vcn_example"
  tier             = "dev"
  create_agent_key = true
}

output "agent_key" {
  value     = warpstream_virtual_cluster_v2.example.agent_key.key
  sensitive = true
}
```

These flags can only be set when the object is created: changing them later is an error.
To add a key to an existing cluster or workspace, use a separate key resource.

## Rotating a key

Replacing a key resource creates a new key with a new secret:

```shell
terraform apply -replace=warpstream_agent_key_v2.example
```

By default Terraform deletes the old key before creating the new one, so anything still using the old key
stops working until it gets the new one. To keep the old key until the new one exists, change the key's `name`
and set `create_before_destroy`:

```terraform
resource "warpstream_agent_key_v2" "example" {
  name               = "akn_example_2"
  virtual_cluster_id = warpstream_virtual_cluster_v2.example.id

  lifecycle {
    create_before_destroy = true
  }
}
```

A key deleted outside Terraform is removed from state, and the next apply creates a new one.

## Importing

The v2 cluster and workspace resources can be imported, but the key's secret can't be.
An imported object has no key in state, so if `create_agent_key` or `create_application_key` is true,
the next apply creates a new key.

The v2 key resources can't be imported.

## Data sources

Data sources that list keys, such as `warpstream_agent_keys`, `warpstream_application_keys` and
`warpstream_virtual_cluster`, list keys in the v2 key format with an empty `key`. Use the `key` attribute of the resource that created the key instead.
