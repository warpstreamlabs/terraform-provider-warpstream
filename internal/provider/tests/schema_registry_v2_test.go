package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
)

func TestAccSchemaRegistryV2Resource(t *testing.T) {
	name := "vcn_sr_test_v2_" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	testAccV2ResourceKey(t, providerConfig+fmt.Sprintf(`
resource "warpstream_schema_registry_v2" "test" {
  name = %q
  tier = "dev"
}`, name), "warpstream_schema_registry_v2.test", "agent_key", name)
}
