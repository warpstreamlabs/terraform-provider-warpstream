package tests

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"

	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// TestAccVirtualClustersDataSource checks for expected attributes on the virtual_clusters data source.
func TestAccVirtualClustersDataSource(t *testing.T) {
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
				Config: testAccVirtualClustersDataSource_default(),
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckVirtualClustersState(vc),
				),
			},
		},
	})
}

func testAccVirtualClustersDataSource_default() string {
	return providerConfig + `
data "warpstream_virtual_clusters" "test" {
}`
}

/*
testCheckVirtualClustersState is a helper to check the state of the virtual clusters data source.
We can't expect a fixed list of virtual clusters in CI since we run tests in parallel and the virtual cluster
resource test suite creates virtual clusters.
There must be a better way to deserialize the data source's attributes but I couldn't figure it out from the docs.
https://developer.hashicorp.com/terraform/plugin/sdkv2/testing/acceptance-tests/teststep#custom-check-functions
*/
func testCheckVirtualClustersState(vc *api.VirtualCluster) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceName := "data.warpstream_virtual_clusters.test"
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Could not find %s resource in root module", resourceName)
		}

		vcs, err := attributesMapToVCStatesSlice(rs.Primary.Attributes)

		if err != nil {
			return err
		}

		err = assertBYOCVC(vcs, vc)
		if err != nil {
			return err
		}

		return nil
	}
}

func assertBYOCVC(vcs []map[string]string, expectedVc *api.VirtualCluster) error {
	vc, err := getVCWithName(vcs, expectedVc.Name)
	if err != nil {
		return err
	}

	if vc["type"] != "byoc" {
		return fmt.Errorf("Expected BYOC virtual cluster, got %s", vc["type"])
	}

	if !strings.HasPrefix(vc["agent_pool_name"], expectedVc.AgentPoolName) {
		return fmt.Errorf("Expected agent pool name to start with 'apn_%s', got %s", expectedVc.Name, vc["agent_pool_name"])
	}

	agentKeysCountAttr, ok := vc["agent_keys.#"]
	if !ok {
		return errors.New("Expected BYOC cluter to have agent keys")
	}
	if agentKeysCountAttr != "1" {
		return fmt.Errorf("Expected 1 agent key, got %s", agentKeysCountAttr)
	}
	agentKeyNameAttr, ok := vc["agent_keys.0.name"]
	if !ok {
		return errors.New("Expected agent key name")
	}

	expectedAgentKeyName := ""
	if expectedVc.AgentKeys != nil {
		agentKeys := *expectedVc.AgentKeys
		if len(agentKeys) > 0 {
			expectedAgentKeyName = agentKeys[0].Name
		}
	}
	if agentKeyNameAttr != expectedAgentKeyName {
		return fmt.Errorf("Expected agent key name to be '%s', got %s", expectedAgentKeyName, agentKeyNameAttr)
	}

	agentKeysVCIDAttr, ok := vc["agent_keys.0.virtual_cluster_id"]
	if !ok {
		return errors.New("Expected agent key virtual cluster ID")
	}
	if agentKeysVCIDAttr != expectedVc.ID {
		return fmt.Errorf("Expected agent key virtual cluster ID to be '%s', got %s", expectedVc.ID, agentKeysVCIDAttr)
	}

	burl, ok := vc["bootstrap_url"]
	if !ok {
		return fmt.Errorf("Expected byoc virtual cluster JSON to have a bootstrap URL field")
	}

	if burl != *expectedVc.BootstrapURL {
		return fmt.Errorf(
			"Expected byoc cluster bootstrap URL to be %s, got %s",
			*expectedVc.BootstrapURL,
			vc["bootstrap_url"],
		)
	}

	workspaceID, ok := vc["workspace_id"]
	if !ok {
		return fmt.Errorf("Expected byoc virtual cluster JSON to have a workspace ID field")
	}
	if workspaceID != expectedVc.WorkspaceID {
		return fmt.Errorf("Expected byoc cluster workspace ID to be %s, got %s", expectedVc.WorkspaceID, workspaceID)
	}

	return nil
}

func getVCWithName(vcs []map[string]string, name string) (map[string]string, error) {
	for _, vc := range vcs {
		if vc["name"] == name {
			return vc, nil
		}
	}
	return nil, fmt.Errorf("No virtual cluster with name %s found", name)
}

/*
attributesMapToVCStatesSlice is a helper to convert the virtual_clusters data source attributes to a slice of
virtual cluster states. TF probably has a better way to do this but I couldn't figure it out from the docs.

	In: map[string]string{
		"virtual_clusters.3.agent_pool_name": "apn_default_80hc",
		"virtual_clusters.1.name": "vcn_streambased",
	}

	Out: []map[string]string{{"agent_pool_name": "apn_default_80hc"}, {"name": "vcn_streambased"}}
*/
func attributesMapToVCStatesSlice(attrsSlice map[string]string) ([]map[string]string, error) {
	vcsMap := make(map[byte]map[string]string)
	for k, v := range attrsSlice { // k = "virtual_clusters.1.name", v = "vcn_streambased"
		if k == "%" { // "%" added by TF to represent a map's length.
			continue
		}

		suffixedAttribute, found := strings.CutPrefix(k, "virtual_clusters.")
		if !found {
			return nil, fmt.Errorf("Unexpected attribute: %s", k)
		}

		if suffixedAttribute == "#" { // "#" added by TF to represent a list's length.
			continue
		}

		vcKey := suffixedAttribute[0]       // Some byte representing "0" to however many VCs we have.
		vcAttrName := suffixedAttribute[2:] // E.g. "name"
		if _, ok := vcsMap[vcKey]; !ok {
			vcsMap[vcKey] = map[string]string{
				vcAttrName: v,
			}
		} else {
			vcsMap[vcKey][vcAttrName] = v
		}
	}

	vcs := make([]map[string]string, 0, len(vcsMap))
	for _, vc := range vcsMap {
		vcs = append(vcs, vc)
	}

	return vcs, nil
}
