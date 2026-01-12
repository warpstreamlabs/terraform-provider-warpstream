# ACL can be imported by specifying the attributes of the Warpstream ACL in the specified format.
# Format: virtual_cluster_id/resource_type/resource_name/pattern_type/principal/host/operation/permission_type
terraform import warpstream_acl.example vci_XXXXXXXXXX/TOPIC/orders/LITERAL/User:alice/*/READ/ALLOW
