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

resource "warpstream_virtual_cluster" "tf_example_wif" {
  name = "vcn_tf_example_wif"
  tier = "dev"
}

# AWS: agents authenticate with an STS GetWebIdentityToken instead of a long-lived agent key,
# matched to their IAM role. All claim match rules must match for a token to be accepted.
resource "warpstream_workload_identity_federation" "aws" {
  virtual_cluster_id         = warpstream_virtual_cluster.tf_example_wif.id
  name                       = "aws-agents"
  issuer_url                 = "https://a1cd7fb2-71fa-411b-b898-17a2055d0f30.tokens.sts.global.api.aws"
  read_only                  = false
  max_credential_ttl_seconds = 3600

  claim_match_rules = [
    {
      claim_path     = "sub"
      expected_value = "arn:aws:iam::123456789012:role/warp-agent"
    },
  ]
}

# GCP: agents authenticate with a Google-signed identity token from the metadata server, matched to
# their service account's email.
resource "warpstream_workload_identity_federation" "gcp" {
  virtual_cluster_id         = warpstream_virtual_cluster.tf_example_wif.id
  name                       = "gcp-agents"
  issuer_url                 = "https://accounts.google.com"
  read_only                  = false
  max_credential_ttl_seconds = 3600

  claim_match_rules = [
    {
      claim_path     = "email"
      expected_value = "warp-agent@my-project.iam.gserviceaccount.com"
    },
  ]
}
