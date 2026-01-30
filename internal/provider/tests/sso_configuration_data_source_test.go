package tests

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAccountKeySequentialSSOConfigurationDataSource(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	userRole, err := client.FindUserRole("Admin")
	require.NoError(t, err)

	ssoConfig, err := client.GetSSOConfigurationWithoutID()
	require.NoError(t, err)
	if ssoConfig != nil {
		err = client.DeleteSSOConfiguration(ssoConfig.ID)
		require.NoError(t, err)
	}

	id, err := client.CreateSSOConfiguration(api.SSOConfigurationCreateRequest{
		EntityID:           "test-entity-id",
		SAMLURL:            "https://example.com/saml",
		DefaultRoleID:      userRole.ID,
		SSOIdentifier:      fmt.Sprintf("test-sso-identifier-%s", uuid.New().String()),
		SigningCertificate: signingCertificate,
	})
	require.NoError(t, err)

	defer func() {
		err = client.DeleteSSOConfiguration(id)
		require.NoError(t, err)
	}()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSSOConfigurationDataSourceWithID(),
				Check:  testAccSSOConfigurationDataSourceCheck(id),
			},
		},
	})
}

func testAccSSOConfigurationDataSourceWithID() string {
	return providerConfig + `
data "warpstream_sso_configuration" "test" {
}`
}

func testAccSSOConfigurationDataSourceCheck(id string) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.warpstream_sso_configuration.test", "id", id),
		resource.TestCheckResourceAttr("data.warpstream_sso_configuration.test", "entity_id", "test-entity-id"),
		resource.TestCheckResourceAttr("data.warpstream_sso_configuration.test", "saml_url", "https://example.com/saml"),
	)
}
