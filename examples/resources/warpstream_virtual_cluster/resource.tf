resource "warpstream_virtual_cluster" "test" {
  name = "vcn_test"
  tier = "dev"
}

resource "warpstream_virtual_cluster" "test_with_acl_shadowing" {
  name = "vcn_test_acl_shadowing"
  tier = "dev"
  configuration = {
    enable_acl_shadowing = true
  }
}

resource "warpstream_virtual_cluster" "test_with_acls" {
  name = "vcn_test_acls"
  tier = "dev"
  configuration = {
    enable_acls = true
  }
}

resource "warpstream_virtual_cluster" "test_configuration" {
  name = "vcn_test_configuration"
  tier = "dev"
  configuration = {
    auto_create_topic        = true
    default_num_partitions   = 1
    default_retention_millis = 86400000
    enable_acls              = true
  }
}

resource "warpstream_virtual_cluster" "test_soft_deletion" {
  name = "vcn_test_soft_deletion"
  tier = "dev"
  configuration = {
    enable_soft_topic_deletion     = true
    soft_topic_deletion_ttl_millis = 172800000
  }
}

resource "warpstream_virtual_cluster" "test_broker_config" {
  name = "vcn_test_broker_config"
  tier = "dev"

  # broker_configuration is the canonical, recommended way to set broker
  # settings: a map of Kafka-style config names to string values. Values must be
  # written exactly as the API reports them ("true", not "T"; "-1" for infinite
  # retention), and only canonical names are accepted (log.retention.ms, never
  # log.retention.hours).
  #
  # Removing a key does not reset the setting on the cluster, because the API has
  # no way to revert a config to its default. Set the value you want instead.
  broker_configuration = {
    "message.max.bytes"   = "1048576"
    "delete.topic.enable" = "true"
    "log.retention.ms"    = "604800000"
  }
}

resource "warpstream_virtual_cluster" "test_broker_config_migration" {
  name = "vcn_test_broker_config_migration"
  tier = "dev"

  # A setting with a deprecated typed attribute may be specified through both
  # surfaces while the values agree, so a module can adopt broker_configuration
  # before dropping its typed attributes. Setting them to different values is
  # rejected at plan time.
  configuration = {
    default_retention_millis = 604800000
  }

  broker_configuration = {
    "log.retention.ms" = "604800000"
  }
}

resource "warpstream_virtual_cluster" "test_cloud_region" {
  name = "vcn_test_cloud_region"
  tier = "dev"
  cloud = {
    provider = "aws"
    region   = "ap-southeast-1"
  }
}

resource "warpstream_virtual_cluster" "test_with_events" {
  name = "vcn_test_with_events"
  tier = "dev"
  events = {
    enabled = true
    event_types = {
      acl_logs = {
        enabled                = true
        retention_period_nanos = 604800000000000 # 7 days in nanoseconds
      }
      pipeline_logs = {
        enabled                = false
        retention_period_nanos = 259200000000000 # 3 days in nanoseconds
      }
    }
  }
}
