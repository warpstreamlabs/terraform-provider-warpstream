package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccWorkloadIdentityFederationResource(t *testing.T) {
	vcName := "vcn_wif_" + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkloadIdentityFederationResource(vcName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_workload_identity_federation.test", "id"),
					resource.TestCheckResourceAttrSet("warpstream_workload_identity_federation.test", "virtual_cluster_id"),
					resource.TestCheckResourceAttr("warpstream_workload_identity_federation.test", "name", "aws-agents"),
					resource.TestCheckResourceAttr("warpstream_workload_identity_federation.test", "issuer_url", "https://oidc.example.com"),
					resource.TestCheckResourceAttr("warpstream_workload_identity_federation.test", "read_only", "false"),
					resource.TestCheckResourceAttr("warpstream_workload_identity_federation.test", "max_credential_ttl_seconds", "3600"),
					resource.TestCheckResourceAttr("warpstream_workload_identity_federation.test", "claim_match_rules.#", "1"),
					resource.TestCheckResourceAttr("warpstream_workload_identity_federation.test", "claim_match_rules.0.claim_path", "sub"),
					resource.TestCheckResourceAttr("warpstream_workload_identity_federation.test", "claim_match_rules.0.expected_value", "arn:aws:iam::123456789012:role/warp-agent"),
					// Audience is derived from the virtual cluster ID by the control plane, not set by the config.
					resource.TestCheckResourceAttrPair(
						"warpstream_workload_identity_federation.test", "audience",
						"warpstream_workload_identity_federation.test", "virtual_cluster_id",
					),
					resource.TestCheckResourceAttrSet("warpstream_workload_identity_federation.test", "created_at"),
				),
			},
			{
				ResourceName:      "warpstream_workload_identity_federation.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					rs := state.RootModule().Resources["warpstream_workload_identity_federation.test"]
					if rs == nil {
						return "", fmt.Errorf("workload identity federation resource not found in state")
					}
					attrs := rs.Primary.Attributes
					return fmt.Sprintf("%s/%s", attrs["virtual_cluster_id"], attrs["id"]), nil
				},
			},
		},
	})
}

func testAccWorkloadIdentityFederationResource(vcName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "wif_vc" {
  name = "%s"
  tier = "dev"
}

resource "warpstream_workload_identity_federation" "test" {
  virtual_cluster_id         = warpstream_virtual_cluster.wif_vc.id
  name                       = "aws-agents"
  issuer_url                 = "https://oidc.example.com"
  read_only                  = false
  max_credential_ttl_seconds = 3600

  claim_match_rules = [
    {
      claim_path     = "sub"
      expected_value = "arn:aws:iam::123456789012:role/warp-agent"
    },
  ]
}
`, vcName)
}
