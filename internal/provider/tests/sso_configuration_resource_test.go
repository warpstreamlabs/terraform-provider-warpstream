package tests

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const (
	signingCertificate = `
-----BEGIN CERTIFICATE-----
MIIBezCCASGgAwIBAgIRAJR1CDT7LFcGH5Ng2cmEjbowCgYIKoZIzj0EAwIwDjEM
MAoGA1UEAxMDZm9vMB4XDTI1MDEwNjIyMjY0OFoXDTI1MDEwNzIyMjY0OFowDjEM
MAoGA1UEAxMDZm9vMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEnIgCPCTQm6FU
xriWAYFvSjMJA81Y81A3KNnfMoqd2aIx0KRKSBCWk/c9RanVRDQJroWK9SWSsYEZ
2mf0lR1S4KNgMF4wDgYDVR0PAQH/BAQDAgeAMB0GA1UdJQQWMBQGCCsGAQUFBwMB
BggrBgEFBQcDAjAdBgNVHQ4EFgQUGlHohetoxg+FkYHMp4ctVtmejjMwDgYDVR0R
BAcwBYIDZm9vMAoGCCqGSM49BAMCA0gAMEUCIB2/O4G7WFiFp3N8EpCS2JpabfhJ
uhsPq7dQR7eCEQAYAiEAg5cP5C73BW8W8MaMVMYifHejaYer9QxLp739hege728=
-----END CERTIFICATE-----
`
)

func TestAccAccountKeySequentialSSOConfigurationResource(t *testing.T) {
	ssoIdentifierSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	client, err := api.NewClientDefault()
	require.NoError(t, err)
	ssoConfig, err := client.GetSSOConfigurationWithoutID()
	require.NoError(t, err)
	if ssoConfig != nil {
		err = client.DeleteSSOConfiguration(ssoConfig.ID)
		require.NoError(t, err)
	}

	userRole, err := client.FindUserRole("Admin")
	require.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSSOConfigurationResource(userRole.ID, ssoIdentifierSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("warpstream_sso_configuration.test", "id"),
					resource.TestCheckResourceAttr("warpstream_sso_configuration.test", "entity_id", "test-entity-id"),
					resource.TestCheckResourceAttr("warpstream_sso_configuration.test", "sso_identifier", fmt.Sprintf("sso-coincoin-%s", ssoIdentifierSuffix)),
				),
			},
			{
				Config: testAccSSOConfigurationResource(userRole.ID, fmt.Sprintf("%s-renamed", ssoIdentifierSuffix)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("warpstream_sso_configuration.test", "sso_identifier", fmt.Sprintf("sso-coincoin-%s-renamed", ssoIdentifierSuffix)),
				),
			},
		},
	})
}

func TestAccAccountKeySequentialSSOConfigurationImport(t *testing.T) {
	ssoIdentifierSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	client, err := api.NewClientDefault()
	require.NoError(t, err)
	ssoConfig, err := client.GetSSOConfigurationWithoutID()
	require.NoError(t, err)
	if ssoConfig != nil {
		err = client.DeleteSSOConfiguration(ssoConfig.ID)
		require.NoError(t, err)
	}

	userRole, err := client.FindUserRole("Admin")
	require.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSSOConfigurationResource(userRole.ID, ssoIdentifierSuffix),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				ResourceName:      "warpstream_sso_configuration.test",
			},
		},
		IsUnitTest: true,
	})
}

func testAccSSOConfigurationResource(defaultRoleID, ssoIdentifierSuffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_sso_configuration" "test" {
  sso_identifier = "sso-coincoin-%s"
  entity_id = "test-entity-id"
  saml_url = "https://rene-coty.com"
  default_role_id = "%s"
  signing_certificate = <<EOT
%s
EOT
}`, ssoIdentifierSuffix, defaultRoleID, signingCertificate)
}
