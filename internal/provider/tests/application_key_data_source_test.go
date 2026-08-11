package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccApplicationKeyDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyDataSource(),
				Check:  testAccApplicationKeyDataSourceCheck(),
			},
		},
	})
}

func TestAccApplicationKeyDataSourceClusterScoped(t *testing.T) {
	vcName := "vcn_app_key_ds_" + nameSuffix
	clusterScopedKeyName := "akn_test_ds_cluster_scoped_app_key" + nameSuffix
	workspaceScopedKeyName := "akn_test_ds_workspace_scoped_app_key" + nameSuffix

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationKeyDataSourceClusterScoped(vcName, clusterScopedKeyName, workspaceScopedKeyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccApplicationKeyDataSourceFindByName(clusterScopedKeyName, map[string]*string{
						"resource_kind": stringPtr(api.ResourceKindVirtualClusterTopics),
					}, true),
					testAccApplicationKeyDataSourceFindByName(workspaceScopedKeyName, map[string]*string{
						"virtual_cluster_id": nil,
						"resource_kind":      nil,
					}, false),
				),
			},
		},
	})
}

func testAccApplicationKeyDataSource() string {
	return providerConfig + `
data "warpstream_application_keys" "test" {
}`
}

func testAccApplicationKeyDataSourceClusterScoped(vcName, clusterScopedKeyName, workspaceScopedKeyName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "test" {
  name = "%s"
  tier = "dev"
}

resource "warpstream_application_key" "cluster_scoped" {
  name               = "%s"
  virtual_cluster_id = warpstream_virtual_cluster.test.id
  resource_kind      = "%s"
}

resource "warpstream_application_key" "workspace_scoped" {
  name = "%s"
}

data "warpstream_application_keys" "test" {
  depends_on = [
    warpstream_application_key.cluster_scoped,
    warpstream_application_key.workspace_scoped,
  ]
}
`, vcName, clusterScopedKeyName, api.ResourceKindVirtualClusterTopics, workspaceScopedKeyName)
}

func testAccApplicationKeyDataSourceCheck() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("data.warpstream_application_keys.test", "application_keys.#"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_application_keys.test", "application_keys.0.name", "akn_"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_application_keys.test", "application_keys.0.key", "aks_"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_application_keys.test", "application_keys.0.workspace_id", "wi_"),
	)
}

// testAccApplicationKeyDataSourceFindByName finds an application key in the datasource list by name
// and asserts expected attributes. A nil value in expectedAttrs means the attribute must be absent.
// If checkVirtualClusterIDPair is true, also asserts virtual_cluster_id matches the VC resource.
func testAccApplicationKeyDataSourceFindByName(keyName string, expectedAttrs map[string]*string, checkVirtualClusterIDPair bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["data.warpstream_application_keys.test"]
		if !ok {
			return fmt.Errorf("data.warpstream_application_keys.test not found in state")
		}

		countStr, ok := rs.Primary.Attributes["application_keys.#"]
		if !ok {
			return fmt.Errorf("application_keys.# not found")
		}

		var count int
		if _, err := fmt.Sscanf(countStr, "%d", &count); err != nil {
			return fmt.Errorf("failed to parse application_keys.#: %w", err)
		}

		index := -1
		for i := 0; i < count; i++ {
			nameAttr := fmt.Sprintf("application_keys.%d.name", i)
			if rs.Primary.Attributes[nameAttr] == keyName {
				index = i
				break
			}
		}
		if index < 0 {
			return fmt.Errorf("application key %q not found in data.warpstream_application_keys.test", keyName)
		}

		for attr, want := range expectedAttrs {
			attrPath := fmt.Sprintf("application_keys.%d.%s", index, attr)
			got, present := rs.Primary.Attributes[attrPath]
			if want == nil {
				if present && got != "" {
					return fmt.Errorf("expected %s to be absent/null for key %q, got %q", attrPath, keyName, got)
				}
				continue
			}
			if !present {
				return fmt.Errorf("expected %s to be set for key %q", attrPath, keyName)
			}
			if got != *want {
				return fmt.Errorf("expected %s = %q for key %q, got %q", attrPath, *want, keyName, got)
			}
		}

		if checkVirtualClusterIDPair {
			vcRS, ok := s.RootModule().Resources["warpstream_virtual_cluster.test"]
			if !ok {
				return fmt.Errorf("warpstream_virtual_cluster.test not found in state")
			}
			vcID := vcRS.Primary.Attributes["id"]
			attrPath := fmt.Sprintf("application_keys.%d.virtual_cluster_id", index)
			got, present := rs.Primary.Attributes[attrPath]
			if !present {
				return fmt.Errorf("expected %s to be set for key %q", attrPath, keyName)
			}
			if got != vcID {
				return fmt.Errorf("expected %s = %q for key %q, got %q", attrPath, vcID, keyName, got)
			}
		}

		return nil
	}
}

func stringPtr(s string) *string {
	return &s
}
