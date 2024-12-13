package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

func TestAccVirtualClusterDataSource(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	vc, err := client.CreateVirtualCluster(
		vcNameSuffix,
		api.ClusterParameters{
			Type:   types.VirtualClusterTypeBYOC,
			Region: "us-east-1",
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
				Config: testAccVirtualClusterDataSourceWithID(vc.ID),
				Check:  testAccVCDataSourceCheck_byoc(vc),
			},
		},
	})
}

func testAccVirtualClusterDataSourceWithID(id string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_virtual_cluster" "test" {
  id = "%s"
}`, id)
}

func testAccVirtualClusterDataSourceWithName(name string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_virtual_cluster" "test" {
  name = "%s"
}`, name)
}

func testAccVCDataSourceCheck_byoc(vc *api.VirtualCluster) resource.TestCheckFunc {
	agentKeyName := ""
	if vc.AgentKeys != nil {
		agentKeys := *vc.AgentKeys
		if len(agentKeys) > 0 {
			agentKeyName = agentKeys[0].Name
		}
	}

	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "type", "byoc"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "agent_keys.#", "1"),
		resource.TestCheckResourceAttr(
			"data.warpstream_virtual_cluster.test", "agent_keys.0.virtual_cluster_id", vc.ID,
		),
		resource.TestCheckResourceAttr(
			"data.warpstream_virtual_cluster.test", "agent_keys.0.name", agentKeyName,
		),
		resource.TestCheckResourceAttr(
			"data.warpstream_virtual_cluster.test", "bootstrap_url", *vc.BootstrapURL,
		),
		testAccVCDataSourceCheck(vc),
	)
}

func testAccVCDataSourceCheck(vc *api.VirtualCluster) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "id"),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "agent_pool_id"),
		utils.TestCheckResourceAttrStartsWith("data.warpstream_virtual_cluster.test", "agent_pool_name", vc.AgentPoolName),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "created_at"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.provider", "aws"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.region", "us-east-1"),
	)
}

// Verify that the virtual cluster data source doesn't work with schema registry clusters.
func TestAccVirtualClusterDatasource_SchemaRegistryNotWork(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	vc, err := client.CreateVirtualCluster(
		vcNameSuffix,
		api.ClusterParameters{
			Type:   types.VirtualClusterTypeSchemaRegistry,
			Region: "us-east-1",
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
				Config:      testAccVirtualClusterDataSourceWithID(vc.ID),
				ExpectError: regexp.MustCompile("must not start with: vci_sr_"),
			},
			{
				Config:      testAccVirtualClusterDataSourceWithName(vc.Name),
				ExpectError: regexp.MustCompile("must not start with: vcn_sr_"),
			},
		},
	})
}
