terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

provider "warpstream" {
  token = "aks_xxx"
}

resource "warpstream_user_role" "example_user_role" {
  name = "example-user-role"

  access_grants = [
    {
      workspace_id = "wi_71e79f1a_ccf4_4836_9b4f_18c11dba9684"
      grant_type   = "admin"
    },
    {
      workspace_id = "wi_bd86729f_6520_49be_97e5_d65b11f978f6"
      grant_type   = "read_only"
    },
  ]
}
