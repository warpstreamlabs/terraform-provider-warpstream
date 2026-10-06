# Overview

The example in main.tf creates application keys with `warpstream_application_key_v2`: a read-only key for the provider key's workspace, and a key that can only manage the topics of one cluster.

Unlike `warpstream_application_key`, the v2 resource creates the key in the v2 key format. The raw key is only returned when it is created, so Terraform keeps it in state as `key`, and that is the only copy. Keep your state in a secure backend.

If a key is deleted outside Terraform, the next `terraform apply` creates a new one.
