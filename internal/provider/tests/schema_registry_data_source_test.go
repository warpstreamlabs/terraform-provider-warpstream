package tests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func TestAccSchemaRegistryDataSource(t *testing.T) {
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

	datasourceName := "data.warpstream_schema_registry.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testSchemaRegistryDataSourceWithID(vc.ID),
				Check:  testAccSchemaRegistryDatasourceCheck(vc, datasourceName),
			},
		},
	})
}

func testSchemaRegistryDataSourceWithID(id string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_schema_registry" "test" {
  id = "%s"
}`, id)
}

func testSchemaRegistryDataSourceWithName(name string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_schema_registry" "test" {
  name = "%s"
}`, name)
}

func testAccSchemaRegistryDatasourceCheck(
	vc *api.VirtualCluster,
	datasourceName string,
) resource.TestCheckFunc {
	agentKeyName := ""
	if vc.AgentKeys != nil {
		agentKeys := *vc.AgentKeys
		if len(agentKeys) > 0 {
			agentKeyName = agentKeys[0].Name
		}
	}

	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(datasourceName, "id"),
		resource.TestCheckResourceAttr(datasourceName, "id", vc.ID),
		resource.TestCheckResourceAttrSet(datasourceName, "created_at"),
		resource.TestCheckResourceAttr(datasourceName, "cloud.provider", "aws"),
		resource.TestCheckResourceAttr(datasourceName, "cloud.region", "us-east-1"),
		resource.TestCheckResourceAttr(datasourceName, "agent_keys.#", "1"),
		resource.TestCheckResourceAttr(datasourceName, "agent_keys.0.virtual_cluster_id", vc.ID),
		resource.TestCheckResourceAttr(datasourceName, "agent_keys.0.name", agentKeyName),
		resource.TestCheckResourceAttr(datasourceName, "bootstrap_url", *vc.BootstrapURL),
		resource.TestCheckResourceAttr(datasourceName, "workspace_id", vc.WorkspaceID),
	)
}

// This test makes sure that you cannot use BYOC virtual cluster's ID/name for schema registry datasource.
func TestAccSchemaRegistryDatasource_BYOCNotWork(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	region := "us-east-1"
	vc, err := client.CreateVirtualCluster(
		vcNameSuffix,
		api.ClusterParameters{
			Type:   api.VirtualClusterTypeBYOC,
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

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testSchemaRegistryDataSourceWithID(vc.ID),
				ExpectError: regexp.MustCompile("must start with 'vci_sr_'"),
			},
			{
				Config:      testSchemaRegistryDataSourceWithName(vc.Name),
				ExpectError: regexp.MustCompile(" must start with 'vcn_sr_'"),
			},
		},
	})
}
