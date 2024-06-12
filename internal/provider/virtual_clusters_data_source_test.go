package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

/*
TestAccVirtualClustersDataSource checks for expected attributes on the virtual_clusters data source.

TODO: Some checks we should run, e.g. on the number of virtual clusters, fail in CI. Probably because of
test parallelism. The virtual cluster resource test suite creates virtual clusters so this suite can't
expect a fixed number of virtual clusters.

Work around this by writing custom check functions for the virtual_cluster data source.
https://developer.hashicorp.com/terraform/plugin/sdkv2/testing/acceptance-tests/teststep#custom-check-functions
*/
func TestAccVirtualClustersDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccVirtualClustersDataSource_default(),
				Check:  testAccVCsDataSourceCheckServerless("tivo_serverless"),
			},
			{
				Config: testAccVirtualClustersDataSource_default(),
				Check:  testAccVCsDataSourceCheckBYOC("wtf"),
			},
		},
	})
}

func testAccVirtualClustersDataSource_default() string {
	return providerConfig + `
data "warpstream_virtual_clusters" "test" {
}`
}

func testAccVCsDataSourceCheckServerless(serverlessVCName string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		// TODO: See TestAccVirtualClustersDataSource comment.
		// resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.#", "5"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.0.name", "vcn_"+serverlessVCName),
		resource.TestCheckResourceAttrWith(
			"data.warpstream_virtual_clusters.test",
			"virtual_clusters.0.agent_pool_name",
			func(value string) error {
				if !strings.HasPrefix(value, "apn_"+serverlessVCName) {
					return fmt.Errorf("expected agent_pool_name to start with 'apn_%s', got: %s", serverlessVCName, value)
				}
				return nil
			},
		),
		resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.0.type", "serverless"),
		// No agent keys in serverless clusters.
		resource.TestCheckNoResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.0.agent_keys"),
	)
}

func testAccVCsDataSourceCheckBYOC(byocVCName string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		// TODO: See TestAccVirtualClustersDataSource comment.
		// resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.#", "5"),
		resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.2.name", "vcn_"+byocVCName),
		resource.TestCheckResourceAttrWith(
			"data.warpstream_virtual_clusters.test",
			"virtual_clusters.2.agent_pool_name",
			func(value string) error {
				if !strings.HasPrefix(value, "apn_"+byocVCName) {
					return fmt.Errorf("expected agent_pool_name to start with 'apn_%s', got: %s", byocVCName, value)
				}
				return nil
			},
		),
		resource.TestCheckResourceAttr("data.warpstream_virtual_clusters.test", "virtual_clusters.2.type", "byoc"),
		resource.TestCheckResourceAttr(
			"data.warpstream_virtual_clusters.test", "virtual_clusters.2.agent_keys.0.name", "akn_virtual_cluster_wtf_af207e45b4e8",
		),
	)
}
