package tests

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider"
)

const (
	// providerConfig is a shared configuration to combine with the actual
	// test configuration so the WarpStream client is properly configured.
	// WARPSTREAM_API_KEY must be set in .github/workflows/test.yml.
	providerConfig = `
provider "warpstream" {
  # base_url = "${WARPSTREAM_API_URL}"
  # token    = "${$WARPSTREAM_API_KEY}"
}
`
)

var (
	// nameSuffix is a random string that we append to resource names
	// in order to prevent name collisions when acceptance tests run
	// in parallel for different terraform versions.
	nameSuffix = acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"warpstream": providerserver.NewProtocol6WithError(provider.New("test")()),
}
