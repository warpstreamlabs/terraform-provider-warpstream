---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "warpstream_virtual_cluster Resource - terraform-provider-warpstream"
subcategory: ""
description: |-
  This resource allows you to create, update and delete virtual clusters.
  The WarpStream provider must be authenticated with an application key to consume this resource.
---

# warpstream_virtual_cluster (Resource)

This resource allows you to create, update and delete virtual clusters.

The WarpStream provider must be authenticated with an application key to consume this resource.

## Example Usage

```terraform
resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
  tier = "dev"
}

resource "warpstream_virtual_cluster" "test_with_acls" {
  name = "vcn_test_acls"
  tier = "dev"
  configuration = {
    enable_acls = true
  }
}

resource "warpstream_virtual_cluster" "test_configuration" {
  name = "vcn_test_configuration"
  tier = "dev"
  configuration = {
    auto_create_topic        = true
    default_num_partitions   = 1
    default_retention_millis = 86400000
    enable_acls              = true
  }
}

resource "warpstream_virtual_cluster" "test_cloud_region" {
  name = "vcn_test_cloud_region"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "ap-southeast-1"
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Virtual Cluster Name.
- `tier` (String) Virtual Cluster Tier. Currently, the valid virtual cluster tiers are `dev`, 'pro', 'fundamentals'.

### Optional

- `cloud` (Attributes) Virtual Cluster Cloud Location. (see [below for nested schema](#nestedatt--cloud))
- `configuration` (Attributes) Virtual Cluster Configuration. (see [below for nested schema](#nestedatt--configuration))
- `tags` (Map of String) Tags associated with the virtual cluster.
- `type` (String) Virtual Cluster Type. Currently, the only valid virtual cluster types is `byoc` (default).

### Read-Only

- `agent_keys` (Attributes List) List of keys to authenticate an agent with this cluster. (see [below for nested schema](#nestedatt--agent_keys))
- `agent_pool_id` (String) Agent Pool ID.
- `agent_pool_name` (String) Agent Pool Name.
- `bootstrap_url` (String) Bootstrap URL to connect to the Virtual Cluster.
- `created_at` (String) Virtual Cluster Creation Timestamp.
- `default` (Boolean)
- `id` (String) Virtual Cluster ID.
- `workspace_id` (String) Workspace ID. ID of the workspace to which the virtual cluster belongs. Assigned based on the workspace of the application key used to authenticate the WarpStream provider. Cannot be changed after creation.

<a id="nestedatt--cloud"></a>
### Nested Schema for `cloud`

Optional:

- `provider` (String) Cloud Provider. Valid providers are: `aws` (default) and `gcp`.
- `region` (String) Cloud Region. Defaults to `us-east-1`. Can't be set if `region_group` is set.
- `region_group` (String) Cloud Region Group. Defaults to null. Can't be set if `region` is set.


<a id="nestedatt--configuration"></a>
### Nested Schema for `configuration`

Optional:

- `auto_create_topic` (Boolean) Enable topic autocreation feature, defaults to `true`.
- `default_num_partitions` (Number) Number of partitions created by default.
- `default_retention_millis` (Number) Default retention for topics that are created automatically using Kafka's topic auto-creation feature.
- `enable_acls` (Boolean) Enable ACLs, defaults to `false`. See [Configure ACLs](https://docs.warpstream.com/warpstream/configuration/configure-acls)
- `enable_deletion_protection` (Boolean) Enable deletion protection, defaults to `false`. If set to true, it is impossible to delete this cluster. enable_deletion_protection needs to be set to false before deleting the cluster.


<a id="nestedatt--agent_keys"></a>
### Nested Schema for `agent_keys`

Read-Only:

- `created_at` (String)
- `id` (String)
- `key` (String, Sensitive)
- `name` (String)
- `virtual_cluster_id` (String)

## Import

Import is supported using the following syntax:

```shell
# Virtual Cluster can be imported by specifying the identifier.
terraform import warpstream_virtual_cluster.example vci_XXXXXXXXXX
```
