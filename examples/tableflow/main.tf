terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

provider "warpstream" {
  # Use a generic WarpStream API key here, not a cluster-specific Agent key.
  token = "aks_dbc98866e6509087e08ed0a9a7de16aa41042853ce66324da55cbb6eff2ffe18"
}

# BYOC cluster with configuration.
resource "warpstream_tableflow_cluster" "example" {
  name = "vcn_dl_example"
  tier = "pro"
  cloud = {
    # This is the cloud provider and region of the WarpStream control plane,
    # *not* the region where the WarpStream Agents are deployed. Agents can
    # be deployed anywhere and should connect to the nearest available
    # WarpStream control plane region.
    provider = "aws"
    region   = "us-east-1"
  }
}

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
          - { name: environment, type: string, id: 1}
          - { name: service, type: string, id: 2}
          - { name: status, type: string, id: 3}
          - { name: message, type: string, id: 4}
destination_bucket_url: s3://tableflow-bucket2
  EOT
}
