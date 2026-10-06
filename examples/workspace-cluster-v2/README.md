# Overview

The example in main.tf creates a WarpStream Workspace with `warpstream_workspace_v2`, then creates a cluster inside that workspace with `warpstream_virtual_cluster_v2`, using the workspace's application key.

Unlike `warpstream_workspace`, the v2 resource creates the workspace's application key (v2 key format) together with the workspace. The raw key is only returned when it is created, so Terraform keeps it in state as `application_key.key`, and that is the only copy. The same applies to the cluster's `agent_key`. Keep your state in a secure backend.

If a key is deleted outside Terraform, the next `terraform apply` creates a new one.
