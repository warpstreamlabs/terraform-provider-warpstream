# Virtual Cluster Credentials can be imported by specifying the virtual cluster ID and credential ID,
# or by specifying the credential ID alone. For the latter, the provider needs to be authenticated
# with a workspace-wide application key (cluster-scoped application keys cannot list virtual clusters).
# NOTE: this won't fetch the value of the `password` field when importing.
terraform import warpstream_virtual_cluster_credentials.example vci_XXXXXXXXXX/cci_XXXXXXXXXX
terraform import warpstream_virtual_cluster_credentials.example cci_XXXXXXXXXX
