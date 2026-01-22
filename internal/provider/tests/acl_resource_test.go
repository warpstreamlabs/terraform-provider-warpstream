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

func TestAccACLResource(t *testing.T) {
	vcName := "vcn_acl_" + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResource(vcName),
				Check:  testAccACLResourceCheck(),
			},
		},
	})
}

func testAccACLResource(vcName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "acl_vc" {
  name = "%s"
  tier = "dev"
  configuration = {
    enable_acls = true
  }
}

resource "warpstream_acl" "test" {
  virtual_cluster_id = warpstream_virtual_cluster.acl_vc.id
  host = "*"
  principal     = "User:alice"
  operation     = "READ"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
  }
`, vcName)
}

func testAccModifiedACLResource(vcName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "acl_vc" {
  name = "%s"
  tier = "dev"
  configuration = {
    enable_acls = true
  }
}

resource "warpstream_acl" "test" {
  virtual_cluster_id = warpstream_virtual_cluster.acl_vc.id
  host = "*"
  principal     = "User:bob"
  operation     = "READ"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
  }
`, vcName)
}

func testAccACLResourceCheck() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_acl.test", "id"),
		resource.TestCheckResourceAttrSet("warpstream_acl.test", "virtual_cluster_id"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "principal", "User:alice"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "resource_type", "TOPIC"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "resource_name", "orders"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "pattern_type", "LITERAL"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "operation", "READ"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "permission_type", "ALLOW"),
	)
}

func TestAccACLResourceDeletePlan(t *testing.T) {
	vcName := "vcn_acl_" + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResource(vcName),
				Check:  testAccACLResourceCheck(),
			},
			{
				PreConfig: func() {
					client, err := api.NewClientDefault()
					require.NoError(t, err)

					vc, err := client.FindVirtualCluster(vcName)
					require.NoError(t, err)

					acls, err := client.ListACLs(vc.ID)
					require.NoError(t, err)

					var aclToDelete string
					for _, acl := range acls {
						if acl.Principal == "User:alice" && acl.ResourceName == "orders" {
							aclToDelete = acl.ID()
							break
						}
					}
					require.NotEmpty(t, aclToDelete)

					err = client.DeleteACL(vc.ID, api.ACLRequest{
						ResourceType:   "TOPIC",
						ResourceName:   "orders",
						PatternType:    "LITERAL",
						Principal:      "User:alice",
						Host:           "*",
						Operation:      "READ",
						PermissionType: "ALLOW",
					})
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_acl.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccACLResourceReplaceOnChange(t *testing.T) {
	vcName := "vcn_acl_" + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	var originalACLID string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResource(vcName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccACLResourceCheck(),
					resource.TestCheckResourceAttrWith("warpstream_acl.test", "id", func(id string) error {
						originalACLID = id
						return nil
					}),
				),
			},
			// Step 2: no-op (same config) -> empty plan
			{
				Config: testAccACLResource(vcName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: change principal -> replacement
			{
				Config: testAccModifiedACLResource(vcName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_acl.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_acl.test", "principal", "User:bob"),
					// Verify ID changed (new resource)
					resource.TestCheckResourceAttrWith("warpstream_acl.test", "id", func(newID string) error {
						if newID == "" {
							return fmt.Errorf("expected new id, got empty")
						}
						if newID == originalACLID {
							return fmt.Errorf("expected replacement with new id; old id=%s new id=%s", originalACLID, newID)
						}
						return nil
					}),
				),
			},
		},
	})
}

func TestAccACLResourceImport(t *testing.T) {
	vcName := "vcn_acl_" + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResource(vcName),
				Check:  testAccACLResourceCheck(),
			},
			{
				ResourceName:      "warpstream_acl.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					// Get the ACL resource from the state
					aclResource := state.RootModule().Resources["warpstream_acl.test"]
					if aclResource == nil {
						return "", fmt.Errorf("ACL resource not found in state")
					}

					attrs := aclResource.Primary.Attributes
					importID := fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s/%s",
						attrs["virtual_cluster_id"],
						attrs["resource_type"],
						attrs["resource_name"],
						attrs["pattern_type"],
						attrs["principal"],
						attrs["host"],
						attrs["operation"],
						attrs["permission_type"],
					)
					return importID, nil
				},
			},
		},
	})
}

func TestAccACLResourceDuplicate(t *testing.T) {
	vcName := "vcn_acl_" + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccACLResourceDuplicate(vcName),
				ExpectError: regexp.MustCompile("Duplicate ACL Configuration"),
			},
		},
	})
}

func testAccACLResourceDuplicate(vcName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "acl_vc" {
  name = "%s"
  tier = "dev"
  configuration = {
    enable_acls = true
  }
}

resource "warpstream_acl" "test1" {
  virtual_cluster_id = warpstream_virtual_cluster.acl_vc.id
  host = "*"
  principal     = "User:alice"
  operation     = "READ"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
}

resource "warpstream_acl" "test2" {
  virtual_cluster_id = warpstream_virtual_cluster.acl_vc.id
  host = "*"
  principal     = "User:alice"
  operation     = "READ"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
}
`, vcName)
}

func TestAccACLResourceMultipleUnique(t *testing.T) {
	vcName := "vcn_acl_" + acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResourceMultipleUnique(vcName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_acl.read", "id"),
					resource.TestCheckResourceAttrSet("warpstream_acl.write", "id"),
					resource.TestCheckResourceAttrSet("warpstream_acl.describe", "id"),
					resource.TestCheckResourceAttr("warpstream_acl.read", "operation", "READ"),
					resource.TestCheckResourceAttr("warpstream_acl.write", "operation", "WRITE"),
					resource.TestCheckResourceAttr("warpstream_acl.describe", "operation", "DESCRIBE"),
				),
			},
		},
	})
}

func testAccACLResourceMultipleUnique(vcName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_virtual_cluster" "acl_vc" {
  name = "%s"
  tier = "dev"
  configuration = {
    enable_acls = true
  }
}

resource "warpstream_acl" "read" {
  virtual_cluster_id = warpstream_virtual_cluster.acl_vc.id
  host = "*"
  principal     = "User:alice"
  operation     = "READ"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
}

resource "warpstream_acl" "write" {
  virtual_cluster_id = warpstream_virtual_cluster.acl_vc.id
  host = "*"
  principal     = "User:alice"
  operation     = "WRITE"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
}

resource "warpstream_acl" "describe" {
  virtual_cluster_id = warpstream_virtual_cluster.acl_vc.id
  host = "*"
  principal     = "User:alice"
  operation     = "DESCRIBE"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
}
`, vcName)
}
