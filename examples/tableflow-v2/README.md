# Overview

The example in main.tf creates a WarpStream Tableflow cluster with `warpstream_tableflow_cluster_v2`, along with a Tableflow pipeline.

Unlike `warpstream_tableflow_cluster`, the v2 resource creates the cluster's agent key (v2 key format) together with the cluster. The raw key is only returned when it is created, so Terraform keeps it in state as `agent_key.key`, and that is the only copy. Keep your state in a secure backend.

If the agent key is deleted outside Terraform, the next `terraform apply` creates a new one.
