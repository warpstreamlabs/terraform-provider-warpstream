package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
)

func TestAccAccountKeyWorkspaceV2Resource(t *testing.T) {
	name := "test_acc_v2_" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	testAccV2ResourceKey(t, providerConfig+fmt.Sprintf(`
resource "warpstream_workspace_v2" "test" {
  name                   = %q
  create_application_key = true
}`, name), "warpstream_workspace_v2.test", "application_key", name)
}

func TestAccWorkspaceV2ResourceWithoutKey(t *testing.T) {
	name := "test_acc_v2_" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	config := `
resource "warpstream_workspace_v2" "test" {
  name = %q
  %s
}`
	testAccV2ResourceWithoutKey(t,
		providerConfig+fmt.Sprintf(config, name, ""),
		providerConfig+fmt.Sprintf(config, name, "create_application_key = true"),
		"warpstream_workspace_v2.test", "application_key", "create_application_key")
}
