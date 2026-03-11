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
