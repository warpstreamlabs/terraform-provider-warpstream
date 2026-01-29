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

resource "warpstream_user_role" "example_user_role_1" {
  name = "example-user-role-admin-and-read-only"

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

resource "warpstream_user_role" "example_user_role_2" {
  name = "example-user-role-billing-only"

  access_grants = [
    {
      workspace_id = "-" // The billing grant type can only be associated with the empty workspace ID and vice-versa.
      grant_type   = "billing"
    },
  ]
}
