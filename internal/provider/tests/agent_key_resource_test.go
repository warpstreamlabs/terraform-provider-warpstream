package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccAgentKeyResourceDeletePlan(t *testing.T) {
	name := "akn_test_agent_key" + nameSuffix
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResource(name, vcID),
				Check:  testAccAgentKeyResourceCheck(name, vcID),
			},
			{
				PreConfig: func() {
					client, err := api.NewClientDefault()
					require.NoError(t, err)

					apiKeys, err := client.GetAPIKeys()
					require.NoError(t, err)

					var apiKeyID string
					for _, apiKey := range apiKeys {
						if apiKey.Name == name {
							apiKeyID = apiKey.ID
							break
						}
					}
					require.NotEmpty(t, apiKeyID)

					err = client.DeleteAPIKey(apiKeyID)
					require.NoError(t, err)
				},
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				RefreshState:       true,
				RefreshPlanChecks: resource.RefreshPlanChecks{
					PostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_agent_key.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccAgentKeyResource(t *testing.T) {
	name := "akn_test_agent_key" + nameSuffix
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResource(name, vcID),
				Check:  testAccAgentKeyResourceCheck(name, vcID),
			},
		},
	})
}

func TestAccAgentKeyResourceSchemaRegistryCluster(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	region := "us-east-1"
	vc, err := client.CreateVirtualCluster(
		vcNameSuffix,
		api.ClusterParameters{
			Type:   api.VirtualClusterTypeSchemaRegistry,
			Tier:   api.VirtualClusterTierPro,
			Region: &region,
			Cloud:  "aws",
		},
	)
	require.NoError(t, err)
	defer func() {
		err := client.DeleteVirtualCluster(vc.ID, vc.Name)
		if err != nil {
			panic(fmt.Errorf("failed to delete virtual cluster: %w", err))
		}
	}()

	name := "akn_test_agent_key" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResource(name, vc.ID),
				Check:  testAccAgentKeyResourceCheck(name, vc.ID),
			},
		},
	})
}

func TestAccAgentKeyResourceTableFlowCluster(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	region := "us-east-1"
	vc, err := client.CreateVirtualCluster(
		vcNameSuffix,
		api.ClusterParameters{
			Type:   api.VirtualClusterTypeTableFlow,
			Tier:   api.VirtualClusterTierPro,
			Region: &region,
			Cloud:  "aws",
		},
	)
	require.NoError(t, err)
	defer func() {
		err := client.DeleteVirtualCluster(vc.ID, vc.Name)
		if err != nil {
			panic(fmt.Errorf("failed to delete virtual cluster: %w", err))
		}
	}()

	name := "akn_test_agent_key" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResource(name, vc.ID),
				Check:  testAccAgentKeyResourceCheck(name, vc.ID),
			},
		},
	})
}

func TestAccAgentKeyResourceReadOnly(t *testing.T) {
	name := "akn_test_agent_key_readonly" + nameSuffix
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResourceWithReadOnly(name, vcID, true),
				Check:  testAccAgentKeyResourceCheckWithReadOnly(name, vcID, "true"),
			},
		},
	})
}

func TestAccAgentKeyResourceNotReadOnly(t *testing.T) {
	name := "akn_test_agent_key_not_readonly" + nameSuffix
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResourceWithReadOnly(name, vcID, false),
				Check:  testAccAgentKeyResourceCheckWithReadOnly(name, vcID, "false"),
			},
		},
	})
}

func TestAccAgentKeyResourceReadOnlyRequiresReplace(t *testing.T) {
	name := "akn_test_agent_key_replace" + nameSuffix
	vcID := "vci_test_virtual_cluster_id"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAgentKeyResourceWithReadOnly(name, vcID, false),
				Check:  testAccAgentKeyResourceCheckWithReadOnly(name, vcID, "false"),
			},
			{
				Config: testAccAgentKeyResourceWithReadOnly(name, vcID, true),
				Check:  testAccAgentKeyResourceCheckWithReadOnly(name, vcID, "true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("warpstream_agent_key.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
			},
		},
	})
}

func testAccAgentKeyResource(name, vcID string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_agent_key" "test" {
  name = "%s"
  virtual_cluster_id = "%s"
}`, name, vcID)
}

func testAccAgentKeyResourceCheck(name, vcID string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "id"),
		resource.TestCheckResourceAttr("warpstream_agent_key.test", "name", name),
		resource.TestCheckResourceAttr("warpstream_agent_key.test", "virtual_cluster_id", vcID),
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "key"),
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "created_at"),
	)
}

func testAccAgentKeyResourceWithReadOnly(name, vcID string, readOnly bool) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_agent_key" "test" {
  name = "%s"
  virtual_cluster_id = "%s"
  read_only = %t
}`, name, vcID, readOnly)
}

func testAccAgentKeyResourceCheckWithReadOnly(name, vcID, readOnly string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "id"),
		resource.TestCheckResourceAttr("warpstream_agent_key.test", "name", name),
		resource.TestCheckResourceAttr("warpstream_agent_key.test", "virtual_cluster_id", vcID),
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "key"),
		resource.TestCheckResourceAttrSet("warpstream_agent_key.test", "created_at"),
		resource.TestCheckResourceAttr("warpstream_agent_key.test", "read_only", readOnly),
	)
}
