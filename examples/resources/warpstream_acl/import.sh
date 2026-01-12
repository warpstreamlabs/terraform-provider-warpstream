# ACL can be imported by specifying the composite identifier.
# Format: virtual_cluster_id/resource_type/resource_name/pattern_type/principal/host/operation/permission_type
terraform import warpstream_acl.example vci_XXXXXXXXXX/TOPIC/orders/LITERAL/User:alice/*/READ/ALLOW
