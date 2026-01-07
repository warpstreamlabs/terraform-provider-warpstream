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

func TestAccTableFlowDataSource(t *testing.T) {
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

	datasourceName := "data.warpstream_tableflow_cluster.test"
	agentKeyName := "akn_test_agent_key" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	defer cleanupAPIKeyByName(t, agentKeyName)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTableFlowDataSourceWithIDAndAgentKey(vc.ID, agentKeyName),
				Check:  testAccTableFlowDatasourceCheck(vc, datasourceName, agentKeyName),
			},
		},
	})
}

func testTableFlowDataSourceWithID(id string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_tableflow_cluster" "test" {
  id = "%s"
}`, id)
}

func testTableFlowDataSourceWithIDAndAgentKey(id, agentKeyName string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_agent_key" "test" {
  name = "%s"
  virtual_cluster_id = "%s"
}

data "warpstream_tableflow_cluster" "test" {
  id = "%s"

  depends_on = [
    warpstream_agent_key.test,
  ]
}`, agentKeyName, id, id)
}

func testTableFlowDataSourceWithName(name string) string {
	return providerConfig + fmt.Sprintf(`
data "warpstream_tableflow_cluster" "test" {
  name = "%s"
}`, name)
}

func testAccTableFlowDatasourceCheck(
	vc *api.VirtualCluster,
	datasourceName string,
	agentKeyName string,
) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrSet(datasourceName, "id"),
		resource.TestCheckResourceAttr(datasourceName, "id", vc.ID),
		resource.TestCheckResourceAttrSet(datasourceName, "created_at"),
		resource.TestCheckResourceAttr(datasourceName, "cloud.provider", "aws"),
		resource.TestCheckResourceAttr(datasourceName, "cloud.region", "us-east-1"),
		resource.TestCheckResourceAttr(datasourceName, "agent_keys.#", "1"),
		resource.TestCheckResourceAttr(datasourceName, "agent_keys.0.virtual_cluster_id", vc.ID),
		resource.TestCheckResourceAttr(datasourceName, "agent_keys.0.name", agentKeyName),
		resource.TestCheckResourceAttr(datasourceName, "workspace_id", vc.WorkspaceID),
	)
}

// This test makes sure that you cannot use BYOC virtual cluster's ID/name for tableflow datasource.
func TestAccTableFlowDatasource_BYOCNotWork(t *testing.T) {
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
				Config:      testTableFlowDataSourceWithID(vc.ID),
				ExpectError: regexp.MustCompile("must start with 'vci_dl_'"),
			},
			{
				Config:      testTableFlowDataSourceWithName(vc.Name),
				ExpectError: regexp.MustCompile(" must start with 'vcn_dl_'"),
			},
		},
	})
}
