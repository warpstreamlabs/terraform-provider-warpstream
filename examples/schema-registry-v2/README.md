# Overview

The example in main.tf creates a WarpStream Schema Registry with `warpstream_schema_registry_v2`.

Unlike `warpstream_schema_registry`, the v2 resource creates the Schema Registry's agent key (v2 key format) together with the Schema Registry. The raw key is only returned when it is created, so Terraform keeps it in state as `agent_key.key`, and that is the only copy. Keep your state in a secure backend.

If the agent key is deleted outside Terraform, the next `terraform apply` creates a new one.
