package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccACLResource(t *testing.T) {
	vcName := "vcn_acl_" + nameSuffix

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

func testAccACLResourceCheck() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_acl.test", "id"),
		resource.TestCheckResourceAttrSet("warpstream_acl.test", "virtual_cluster_id"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "principal", "User:alice"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "resource_type", "TOPIC"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "resource_name", "orders"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "pattern_type", "LITERAL"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "operation", "READ"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "permission", "ALLOW"),
	)
}

func TestAccACLResourceDeletePlan(t *testing.T) {
	vcName := "vcn_acl_" + nameSuffix

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResource(vcName),
				Check:  testAccACLResourceCheck(),
			},
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
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
	vcName := "vcn_acl_" + nameSuffix

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
				Config: providerConfig + `
resource "warpstream_acl" "test" {
  virtual_cluster_id = "warpstream_virtual_cluster.acl_vc.id"
  host = "*"
  principal     = "User:bob"      // changed immutable field
  operation     = "READ"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
}
`,
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
