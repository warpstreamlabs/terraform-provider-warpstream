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
	name := "akn_test_agent_key" + nameSuffix
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResource(name, vcID),
				Check:  testAccACLResourceCheck(name, vcID),
			},
		},
	})
}

func testAccACLResource(name, vcID string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_acl" "%s" {
  virtual_cluster_id = "%s"
  host = "*"
  principal     = "User:alice"
  operation     = "READ"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
  }
`, name, vcID)
}

func testAccACLResourceCheck(name, vcID string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_acl.test", "id"),
		resource.TestCheckResourceAttrSet("warpstream_acl.test", "created_at"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "principal", "User:alice"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "resource_type", "TOPIC"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "resource_name", "orders"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "pattern_type", "LITERAL"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "operation", "READ"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "permission", "ALLOW"),
		resource.TestCheckResourceAttr("warpstream_acl.test", "virtual_cluster_id", vcID),
		resource.TestCheckResourceAttr("warpstream_acl.test", "name", name),
	)
}

func TestAccACLResourceDeletePlan(t *testing.T) {
	name := "akn_test_agent_key" + nameSuffix
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResource(name, vcID),
				Check:  testAccACLResourceCheck(name, vcID),
			},
			{
				PreConfig: func() {
					token := os.Getenv("WARPSTREAM_API_KEY")
					client, err := api.NewClient("", &token)
					require.NoError(t, err)

					acls, err := client.ListACLs(vcID)
					require.NoError(t, err)

					var aclToDelete string
					for _, acl := range acls {
						if acl.Principal == "User:alice" && acl.ResourceName == "orders" {
							aclToDelete = acl.ID
							break
						}
					}
					require.NotEmpty(t, aclToDelete)

					err = client.DeleteACL(vcID, api.ACLRequest{
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
	vcID := "vci_test_virtual_cluster_id"
	originalName := "akn_test_agent_key" + nameSuffix
	updatedName := "akn_test_agent_key_updated" + nameSuffix

	var originalID string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccACLResource(originalName, vcID),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccACLResourceCheck(originalName, vcID),
					resource.TestCheckResourceAttrWith("warpstream_acl.test", "id", func(id string) error {
						originalID = id
						return nil
					}),
				),
			},
			// Step 2: no-op (same config) -> empty plan
			{
				Config: testAccACLResource(originalName, vcID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: change principal -> replacement
			{
				Config: providerConfig + fmt.Sprintf(`
resource "warpstream_acl" "%s" {
  virtual_cluster_id = "%s"
  host = "*"
  principal     = "User:bob"      // changed immutable field
  operation     = "READ"
  permission_type    = "ALLOW"
  resource_type = "TOPIC"
  resource_name = "orders"
  pattern_type  = "LITERAL"
}
`, updatedName, vcID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_acl.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_acl.test", "principal", "User:bob"),
					// Verify ID changed (new resource)
					resource.TestCheckResourceAttrWith("warpstream_acl.test", "id", func(v string) error {
						if v == "" {
							return fmt.Errorf("expected new id, got empty")
						}
						if v == originalID {
							return fmt.Errorf("expected replacement with new id; old id=%s new id=%s", originalID, v)
						}
						return nil
					}),
				),
			},
		},
	})
}
