package provider

import (
	"fmt"
	"testing"
)

func testSchemaRegistryResource(nameSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_schema_registry" "test" {
  name = "vcn_sr_test_acc_%s"
}`, nameSuffix)
}

func TestAccSchemaRegistryResource(t *testing.T) {

}
