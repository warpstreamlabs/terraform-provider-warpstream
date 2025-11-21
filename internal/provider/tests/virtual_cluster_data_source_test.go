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

func TestAccVirtualClusterDataSource(t *testing.T) {
	client, err := api.NewClientDefault()
	require.NoError(t, err)

	vcNameSuffix := acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
	region := "us-east-1"
	vc, err := client.CreateVirtualCluster(
		vcNameSuffix,
		api.ClusterParameters{
			Type:           api.VirtualClusterTypeBYOC,
			Tier:           api.VirtualClusterTierPro,
			Region:         &region,
			Cloud:          "aws",
			Tags:           map[string]string{"test_tag": "test_value"},
			CreateAgentKey: true,
		},
	)
	require.NoError(t, err)
	defer func() {
		err := client.DeleteVirtualCluster(vc.ID, vc.Name)
		if err != nil {
			panic(fmt.Errorf("failed to delete virtual cluster: %w", err))
		}
	}()

	cfg, err := client.GetConfiguration(*vc)
	require.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClusterDataSourceWithID(vc.ID),
				Check:  testAccVCDataSourceCheck_byoc(vc, cfg),
			},
			{
				Config: testAccVirtualClusterDataSourceWithName(vc.Name),
				Check:  testAccVCDataSourceCheck_byoc(vc, cfg),
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

func testAccVCDataSourceCheck_byoc(vc *api.VirtualCluster, cfg *api.VirtualClusterConfiguration) resource.TestCheckFunc {
	agentKeyName := ""
	if vc.AgentKeys != nil {
		agentKeys := *vc.AgentKeys
		if len(agentKeys) > 0 {
			agentKeyName = agentKeys[0].Name
		}
	}

	softTopicDeletionTTL := int64(86400000)
	if cfg.SoftTopicDeletionTTLMillis != nil {
		softTopicDeletionTTL = *cfg.SoftTopicDeletionTTLMillis
	}

	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "type", "byoc"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "tags.test_tag", "test_value"),
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
		resource.TestCheckResourceAttr(
			"data.warpstream_virtual_cluster.test", "configuration.enable_soft_topic_deletion",
			fmt.Sprintf("%t", cfg.EnableSoftTopicDeletion),
		),
		resource.TestCheckResourceAttr(
			"data.warpstream_virtual_cluster.test", "configuration.soft_topic_deletion_ttl_millis",
			fmt.Sprintf("%d", softTopicDeletionTTL),
		),
		testAccVCDataSourceCheck(vc),
	)
}

func testAccVCDataSourceCheck(vc *api.VirtualCluster) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "id", vc.ID),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "agent_pool_id"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "tags.test_tag", "test_value"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "agent_pool_name", vc.AgentPoolName),
		resource.TestCheckResourceAttrSet("data.warpstream_virtual_cluster.test", "created_at"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "workspace_id", vc.WorkspaceID),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.provider", "aws"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_cluster.test", "cloud.region", "us-east-1"),
	)
}

// Verify that the virtual cluster data source doesn't work with schema registry clusters.
func TestAccVirtualClusterDatasource_SchemaRegistryNotWork(t *testing.T) {
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
