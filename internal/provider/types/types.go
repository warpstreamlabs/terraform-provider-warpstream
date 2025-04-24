package types

// TODO simon: move to models package
const (
	VirtualClusterTypeBYOC           = "byoc"
	VirtualClusterTypeSchemaRegistry = "byoc_schema_registry"

	// legacy is only available for certain tenants, this is controlled on the Warpstream side.
	VirtualClusterTierLegacy       = "legacy"
	VirtualClusterTierDev          = "dev"
	VirtualClusterTierFundamentals = "fundamentals"
	VirtualClusterTierPro          = "pro"
)
