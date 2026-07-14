# A workload identity federation binding can be imported using its virtual cluster ID and its ID.
# Format: virtual_cluster_id/workload_identity_federation_id
terraform import warpstream_workload_identity_federation.example 'vci_XXXXXXXXXX/wif_XXXXXXXXXX'
