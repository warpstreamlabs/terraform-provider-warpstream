package tests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAccountKeyUserRoleResource(t *testing.T) {
	randSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserRoleResource(randSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "name", fmt.Sprintf("test_acc_user_role_%s", randSuffix)),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "created_at"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "access_grants.#", "1"),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "access_grants.0.workspace_id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "access_grants.0.grant_type", "admin"),
				),
			},
			{
				Config: testAccUserRoleResourceUpdate(randSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "name", fmt.Sprintf("test_acc_user_role_updated_%s", randSuffix)),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "created_at"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "access_grants.#", "2"),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "access_grants.0.workspace_id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "access_grants.0.grant_type", "read_only"),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test", "access_grants.1.workspace_id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test", "access_grants.1.grant_type", "admin"),
				),
			},
		},
	})
}

func testAccUserRoleResource(randSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_workspace" "test" {
  name = "test_acc_workspace_%s"
}

resource "warpstream_user_role" "test" {
  name = "test_acc_user_role_%s"
  access_grants = [
    {
      workspace_id = warpstream_workspace.test.id
      grant_type   = "admin"
    },
  ]
}`, randSuffix, randSuffix)
}

func testAccUserRoleResourceUpdate(randSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_workspace" "test" {
  name = "test_acc_workspace_%s"
}

resource "warpstream_workspace" "test2" {
  name = "test_acc_workspace_2_%s"
}

resource "warpstream_user_role" "test" {
  name = "test_acc_user_role_updated_%s"
  access_grants = [
    {
      workspace_id = warpstream_workspace.test.id
      grant_type   = "read_only"
    },
    {
      workspace_id = warpstream_workspace.test2.id
      grant_type   = "admin"
    },
  ]
}`, randSuffix, randSuffix, randSuffix)
}

func TestAccAccountKeyUserRoleResourceWithBillingGrant(t *testing.T) {
	randSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserRoleResourceWithBillingGrant(randSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_user_role.test_billing", "id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test_billing", "name", fmt.Sprintf("test_acc_user_role_billing_%s", randSuffix)),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test_billing", "created_at"),
					resource.TestCheckResourceAttr("warpstream_user_role.test_billing", "access_grants.#", "2"),
					resource.TestCheckResourceAttrSet("warpstream_user_role.test_billing", "access_grants.0.workspace_id"),
					resource.TestCheckResourceAttr("warpstream_user_role.test_billing", "access_grants.0.grant_type", "admin"),
					resource.TestCheckResourceAttr("warpstream_user_role.test_billing", "access_grants.1.workspace_id", "-"),
					resource.TestCheckResourceAttr("warpstream_user_role.test_billing", "access_grants.1.grant_type", "billing"),
				),
			},
		},
	})
}

func testAccUserRoleResourceWithBillingGrant(randSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_workspace" "test" {
  name = "test_acc_workspace_%s"
}

resource "warpstream_user_role" "test_billing" {
  name = "test_acc_user_role_billing_%s"
  access_grants = [
    {
      workspace_id = warpstream_workspace.test.id
      grant_type   = "admin"
    },
    {
      workspace_id = "-"
      grant_type   = "billing"
    },
  ]
}`, randSuffix, randSuffix)
}

func TestAccAccountKeyUserRoleResourceBillingGrantInvalidWorkspace(t *testing.T) {
	randSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserRoleResourceBillingGrantInvalidWorkspace(randSuffix),
				ExpectError: regexp.MustCompile(`The 'billing' grant type must be assigned with the empty workspace_id '-'.`),
			},
		},
	})
}

func testAccUserRoleResourceBillingGrantInvalidWorkspace(randSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_workspace" "test" {
  name = "test_acc_workspace_%s"
}

resource "warpstream_user_role" "test_billing_invalid" {
  name = "test_acc_user_role_billing_invalid_%s"
  access_grants = [
    {
      workspace_id = warpstream_workspace.test.id
      grant_type   = "billing"
    },
  ]
}`, randSuffix, randSuffix)
}

func TestAccAccountKeyUserRoleResourceNilWorkspaceInvalidGrantType(t *testing.T) {
	randSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserRoleResourceNilWorkspaceInvalidGrantType(randSuffix),
				ExpectError: regexp.MustCompile(`The empty workspace ID '-' can only be assigned with the 'billing' grant`),
			},
		},
	})
}

func testAccUserRoleResourceNilWorkspaceInvalidGrantType(randSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_user_role" "test_nil_workspace_invalid" {
  name = "test_acc_user_role_nil_ws_invalid_%s"
  access_grants = [
    {
      workspace_id = "-"
      grant_type   = "admin"
    },
  ]
}`, randSuffix)
}
