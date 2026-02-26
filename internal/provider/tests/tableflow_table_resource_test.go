package tests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

var (
	tableflowTableClusterSuffix = acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum)
)

func TestTableflowTableResource(t *testing.T) {
	resourceName := "warpstream_tableflow_table.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the tableflow cluster + pipeline so the scheduler creates the table.
			{
				Config: testTableflowTableInfraOnly(tableflowTableClusterSuffix),
			},
			// Step 2: Add the tableflow_table resource to adopt the table.
			{
				Config: testTableflowTableFull(tableflowTableClusterSuffix, "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "table_uuid"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
					resource.TestCheckResourceAttr(resourceName, "recreation_key", "v1"),
				),
			},
			// Step 3: Re-apply with same config; plan should be empty.
			{
				Config: testTableflowTableFull(tableflowTableClusterSuffix, "v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestTableflowTableResourceRecreation(t *testing.T) {
	resourceName := "warpstream_tableflow_table.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the tableflow cluster + pipeline.
			{
				Config: testTableflowTableInfraOnly(tableflowTableClusterSuffix),
			},
			// Step 2: Adopt the table with recreation_key = "v1".
			{
				Config: testTableflowTableFull(tableflowTableClusterSuffix, "v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "table_uuid"),
					resource.TestCheckResourceAttr(resourceName, "recreation_key", "v1"),
				),
			},
			// Step 3: Change recreation_key to "v2"; plan should show replace.
			{
				Config: testTableflowTableFull(tableflowTableClusterSuffix, "v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "table_uuid"),
					resource.TestCheckResourceAttr(resourceName, "recreation_key", "v2"),
				),
			},
		},
	})
}

func testTableflowTableInfraOnly(suffix string) string {
	return providerConfig + fmt.Sprintf(`
resource "warpstream_tableflow_cluster" "test" {
  name = "vcn_dl_tbl_test_%s"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}

resource "warpstream_pipeline" "test_pipeline" {
  virtual_cluster_id = warpstream_tableflow_cluster.test.id
  name               = "test_pipeline"
  state              = "running"
  type               = "tableflow"
  configuration_yaml = <<EOT
source_clusters:
  - name: kafka_cluster_1
    bootstrap_brokers:
      - hostname: localhost
        port: 9092
tables:
  - source_cluster_name: kafka_cluster_1
    source_topic: logs
    source_format: json
    schema_mode: inline
    schema:
      fields:
        - { name: environment, type: string, id: 1 }
        - { name: service, type: string, id: 2 }
destination_bucket_url: s3://test-tableflow-bucket?region=us-east-1
EOT
}
`, suffix)
}

func testTableflowTableFull(suffix, recreationKey string) string {
	return testTableflowTableInfraOnly(suffix) + fmt.Sprintf(`
resource "warpstream_tableflow_table" "test" {
  virtual_cluster_id = warpstream_tableflow_cluster.test.id
  table_name         = "kafka_cluster_1__logs"
  recreation_key     = %q

  depends_on = [warpstream_pipeline.test_pipeline]
}
`, recreationKey)
}
