# Overview

The example in main.tf creates a WarpStream BYOC cluster with `warpstream_virtual_cluster_v2`, along with a topic and client credentials.

Unlike `warpstream_virtual_cluster`, the v2 resource creates the cluster's agent key (v2 key format) together with the cluster. The raw key is only returned when it is created, so Terraform keeps it in state as `agent_key.key`, and that is the only copy. Keep your state in a secure backend.

If the agent key is deleted outside Terraform, the next `terraform apply` creates a new one.
