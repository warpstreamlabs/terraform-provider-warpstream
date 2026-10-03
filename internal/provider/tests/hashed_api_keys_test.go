package tests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

const hashedProviderConfig = `
provider "warpstream" {
  hashed_api_keys = true
}
`

var hashedSecret = regexp.MustCompile(`^aks_v2_`)

func TestAccAgentKeyResourceHashed(t *testing.T) {
	name := "akn_test_hashed_agent_key" + nameSuffix
	config := hashedProviderConfig + fmt.Sprintf(`
resource "warpstream_agent_key" "test" {
  name = "%s"
  virtual_cluster_id = "vci_test_virtual_cluster_id"
}`, name)

	testAccHashedKey(t, config, "warpstream_agent_key.test")
}

func TestAccApplicationKeyResourceHashed(t *testing.T) {
	name := "akn_test_hashed_application_key" + nameSuffix
	config := hashedProviderConfig + fmt.Sprintf(`
resource "warpstream_application_key" "test" {
  name = "%s"
}`, name)

	testAccHashedKey(t, config, "warpstream_application_key.test")
}

// testAccHashedKey checks that the key is created hashed, that the API no longer lists its secret, and
// that a refresh keeps the create-time secret in state.
func testAccHashedKey(t *testing.T, config, resourcePath string) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourcePath, "key", hashedSecret),
					testAccCheckListedSecretEmpty(resourcePath, "id"),
				),
			},
			{
				RefreshState: true,
				Check:        resource.TestMatchResourceAttr(resourcePath, "key", hashedSecret),
			},
		},
	})
}

// testAccCheckListedSecretEmpty checks that list_api_keys doesn't return the secret of the key whose
// ID is in the resource's idAttr attribute.
func testAccCheckListedSecretEmpty(resourcePath, idAttr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourcePath]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourcePath)
		}

		client, err := api.NewClientDefault()
		if err != nil {
			return err
		}
		keyID := rs.Primary.Attributes[idAttr]
		key, err := client.GetAPIKey(keyID)
		if err != nil {
			return err
		}
		if key.Key != "" {
			return fmt.Errorf("hashed key %s is still retrievable from list_api_keys", keyID)
		}
		return nil
	}
}
