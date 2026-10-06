# Overview

The example in main.tf creates a WarpStream BYOC cluster with `warpstream_virtual_cluster_v2`, and an extra read-only agent key for it with `warpstream_agent_key_v2`, for example to read the cluster's hosted Prometheus endpoint.

Unlike `warpstream_agent_key`, the v2 resource creates the key in the v2 key format. The raw key is only returned when it is created, so Terraform keeps it in state as `key`, and that is the only copy. Keep your state in a secure backend.

If the key is deleted outside Terraform, the next `terraform apply` creates a new one.
