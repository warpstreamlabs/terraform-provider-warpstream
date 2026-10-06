package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccApplicationKeyV2Resource(t *testing.T) {
	suffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	testAccKeyV2Resource(t, providerConfig+fmt.Sprintf(`
resource "warpstream_application_key_v2" "test" {
  name      = "akn_test_acc_appv2_%s"
  read_only = true
}`, suffix), "warpstream_application_key_v2.test")
}

func TestAccApplicationKeyV2ResourceClusterScoped(t *testing.T) {
	suffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	testAccKeyV2Resource(t, providerConfig+fmt.Sprintf(`
resource "warpstream_virtual_cluster_v2" "test" {
  name = "vcn_test_acc_appv2_%s"
  tier = "fundamentals"
}

resource "warpstream_application_key_v2" "test" {
  name               = "akn_test_acc_appv2_topics_%s"
  virtual_cluster_id = warpstream_virtual_cluster_v2.test.id
  resource_kind      = %q
}`, suffix, suffix, api.ResourceKindVirtualClusterTopics), "warpstream_application_key_v2.test")
}

// testAccKeyV2Resource checks that a v2 key resource keeps its secret across a refresh, and that a key
// deleted outside Terraform is created again.
func testAccKeyV2Resource(t *testing.T, config, resourcePath string) {
	var secret, keyID string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourcePath, "key", v2KeySecret),
					testAccCheckListedSecretEmpty(resourcePath, "id"),
					func(s *terraform.State) error {
						rs := s.RootModule().Resources[resourcePath]
						secret = rs.Primary.Attributes["key"]
						keyID = rs.Primary.ID
						return nil
					},
				),
			},
			{
				RefreshState: true,
				Check:        resource.TestCheckResourceAttrWith(resourcePath, "key", equals(&secret)),
			},
			{
				PreConfig: func() {
					client, err := api.NewClientDefault()
					require.NoError(t, err)
					require.NoError(t, client.DeleteAPIKey(keyID))
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(resourcePath, plancheck.ResourceActionCreate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourcePath, "key", v2KeySecret),
					resource.TestCheckResourceAttrWith(resourcePath, "key", func(v string) error {
						if v == secret {
							return fmt.Errorf("%s was not created again", resourcePath)
						}
						return nil
					}),
				),
			},
		},
	})
}
