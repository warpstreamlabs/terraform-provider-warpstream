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
  name = %q
}`, name), "warpstream_workspace_v2.test", "application_key", name)
}
