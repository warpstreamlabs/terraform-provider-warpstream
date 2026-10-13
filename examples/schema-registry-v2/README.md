# Overview

The example in main.tf creates a WarpStream Schema Registry with `warpstream_schema_registry_v2`, along with an agent key.

The agent key comes from `warpstream_agent_key_v2`, which creates it in the v2 key format. The raw key is only returned when it is created, so Terraform keeps it in state as `key`, and that is the only copy. Keep your state in a secure backend.

To have the Schema Registry create its own agent key instead, set `create_agent_key = true` and read it from `agent_key.key`.

If the agent key is deleted outside Terraform, the next `terraform apply` creates a new one.
