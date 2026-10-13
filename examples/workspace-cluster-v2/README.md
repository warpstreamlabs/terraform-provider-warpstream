# Overview

The example in main.tf creates a WarpStream Workspace with `warpstream_workspace_v2`, then creates a cluster inside that workspace with `warpstream_virtual_cluster_v2`, using an application key for that workspace.

The application key and the agent key come from `warpstream_application_key_v2` and `warpstream_agent_key_v2`, which create them in the v2 key format. A raw key is only returned when it is created, so Terraform keeps it in state as `key`, and that is the only copy. Keep your state in a secure backend.

To have the workspace or the cluster create its own key instead, set `create_application_key = true` or `create_agent_key = true` and read it from `application_key.key` or `agent_key.key`.

If a key is deleted outside Terraform, the next `terraform apply` creates a new one.
