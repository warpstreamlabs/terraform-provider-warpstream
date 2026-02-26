terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

provider "warpstream" {
  # Use a generic WarpStream API key here, not a cluster-specific Agent key.
  token = "YOUR_API_KEY"
}

# Tableflow cluster.
resource "warpstream_tableflow_cluster" "example" {
  name = "vcn_dl_example"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "us-east-1"
  }
}

# Pipeline that defines the table.
resource "warpstream_pipeline" "tableflow_pipeline" {
  virtual_cluster_id = warpstream_tableflow_cluster.example.id
  name               = "example_tableflow_pipeline"
  state              = "running"
  type               = "tableflow"
  configuration_yaml = <<EOT
source_clusters:
  - name: kafka_cluster_1
    bootstrap_brokers:
      - hostname: YOUR_KAFKA_HOSTNAME
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
        - { name: status, type: string, id: 3 }
        - { name: message, type: string, id: 4 }
destination_bucket_url: s3://tableflow-bucket?region=us-east-1
EOT
}

# Adopt the table created by the scheduler.
# Change recreation_key to force a safe table drop + re-creation.
# Alternatively: terraform apply -replace=warpstream_tableflow_table.logs
resource "warpstream_tableflow_table" "logs" {
  virtual_cluster_id = warpstream_tableflow_cluster.example.id
  table_name         = "kafka_cluster_1__logs"
  recreation_key     = "v1"

  depends_on = [warpstream_pipeline.tableflow_pipeline]
}

output "table_uuid" {
  value = warpstream_tableflow_table.logs.table_uuid
}

output "table_created_at" {
  value = warpstream_tableflow_table.logs.created_at
}
