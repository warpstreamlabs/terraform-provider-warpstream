package tests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

var hashedSecret = regexp.MustCompile(`^aks_v2_`)

func TestAccVirtualClusterV2Resource(t *testing.T) {
	name := "vcn_test_acc_v2_" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	testAccV2ResourceKey(t, providerConfig+fmt.Sprintf(`
resource "warpstream_virtual_cluster_v2" "test" {
  name = %q
  tier = "fundamentals"
}`, name), "warpstream_virtual_cluster_v2.test", "agent_key", name)
}

func TestAccSchemaRegistryV2Resource(t *testing.T) {
	name := "vcn_sr_test_v2_" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	testAccV2ResourceKey(t, providerConfig+fmt.Sprintf(`
resource "warpstream_schema_registry_v2" "test" {
  name = %q
  tier = "dev"
}`, name), "warpstream_schema_registry_v2.test", "agent_key", name)
}

func TestAccTableFlowV2Resource(t *testing.T) {
	name := "vcn_dl_test_v2_" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	testAccV2ResourceKey(t, providerConfig+fmt.Sprintf(`
resource "warpstream_tableflow_cluster_v2" "test" {
  name = %q
  tier = "dev"
}`, name), "warpstream_tableflow_cluster_v2.test", "agent_key", name)
}

func TestAccAccountKeyWorkspaceV2Resource(t *testing.T) {
	name := "test_acc_v2_" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	testAccV2ResourceKey(t, providerConfig+fmt.Sprintf(`
resource "warpstream_workspace_v2" "test" {
  name = %q
}`, name), "warpstream_workspace_v2.test", "application_key", name)
}

// testAccV2ResourceKey checks that a v2 resource stores its hashed key, keeps it across a refresh, and
// replaces it in place when the key is deleted outside Terraform.
func testAccV2ResourceKey(t *testing.T, config, resourcePath, keyAttr, objectName string) {
	var secret string
	var objectID string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourcePath, keyAttr+".key", hashedSecret),
					testAccCheckListedSecretEmpty(resourcePath, keyAttr+".id"),
					func(s *terraform.State) error {
						rs := s.RootModule().Resources[resourcePath]
						secret = rs.Primary.Attributes[keyAttr+".key"]
						objectID = rs.Primary.ID
						return nil
					},
				),
			},
			{
				RefreshState: true,
				Check:        resource.TestCheckResourceAttrWith(resourcePath, keyAttr+".key", equals(&secret)),
			},
			{
				PreConfig: func() { deleteOwnedKey(t, objectID) },
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(resourcePath, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr(resourcePath, keyAttr+".key", hashedSecret),
					resource.TestCheckResourceAttrWith(resourcePath, keyAttr+".key", func(v string) error {
						if v == secret {
							return fmt.Errorf("%s key for %s was not replaced", keyAttr, objectName)
						}
						return nil
					}),
				),
			},
		},
	})
}

func equals(want *string) resource.CheckResourceAttrWithFunc {
	return func(v string) error {
		if v != *want {
			return fmt.Errorf("got a different value than before")
		}
		return nil
	}
}

// deleteOwnedKey deletes the key granted on the cluster or workspace objectID.
func deleteOwnedKey(t *testing.T, objectID string) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)
	keys, err := client.GetAPIKeys()
	require.NoError(t, err)
	for _, key := range keys {
		for _, grant := range key.AccessGrants {
			if grant.ResourceID == objectID || (grant.WorkspaceID == objectID && grant.ResourceKind == api.ResourceKindAny) {
				require.NoError(t, client.DeleteAPIKey(key.ID))
				return
			}
		}
	}
	t.Fatalf("no key found for %s", objectID)
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
