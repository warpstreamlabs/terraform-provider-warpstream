package resources

import "fmt"

// brokerKeyTypedAttr maps a cluster broker config name to the `configuration`
// attribute that controls the same underlying setting.
//
// The two surfaces are disjoint: a setting listed here can only be written through its typed
// attribute, and `broker_configuration` rejects its key at plan time. This set is closed by
// policy — new cluster configs are map-only and must not grow it — so the typed attributes can
// be removed wholesale in the next major version, at which point these keys become legal in
// the map.
var brokerKeyTypedAttr = map[string]string{
	"auto.create.topics.enable":           "auto_create_topic",
	"num.partitions":                      "default_num_partitions",
	"log.retention.ms":                    "default_retention_millis",
	"warpstream.default.topic.type":       "default_topic_type",
	"warpstream.soft.delete.topic.enable": "enable_soft_topic_deletion",
	"warpstream.soft.delete.topic.ttl.ms": "soft_topic_deletion_ttl_millis",
}

// writeOnlyAliasKeys names configs the API accepts on write but never reports back on describe,
// because they are alternate units for a canonical key. A value declared under one of these
// could never be read back, so Terraform would fail comparing state against the configuration.
// Therefore, we don't allow these aliases to be used in Terraform.
var writeOnlyAliasKeys = map[string]string{
	"log.retention.minutes":                  "log.retention.ms",
	"log.retention.hours":                    "log.retention.ms",
	"warpstream.soft.delete.topic.ttl.hours": "warpstream.soft.delete.topic.ttl.ms",
}

// validateBrokerConfigKey rejects the config names `broker_configuration` cannot hold: settings
// controlled by a typed `configuration` attribute, and write-only aliases. Unknown names are
// deliberately not rejected: the API is the authority on which configs exist, and adding one
// there must not require a provider release.
func validateBrokerConfigKey(key string) error {
	if typedAttr, ok := brokerKeyTypedAttr[key]; ok {
		return fmt.Errorf(
			"%q is controlled by the `configuration.%s` attribute in this provider; "+
				"set it there instead",
			key, typedAttr,
		)
	}
	if canonical, ok := writeOnlyAliasKeys[key]; ok {
		// An alias for a setting with a typed attribute redirects there, like the canonical
		// key itself would; anything else redirects to the canonical spelling.
		if typedAttr, mirrored := brokerKeyTypedAttr[canonical]; mirrored {
			return fmt.Errorf(
				"%q is an alternate unit for %q, which is controlled by the "+
					"`configuration.%s` attribute in this provider; set it there instead",
				key, canonical, typedAttr,
			)
		}
		return fmt.Errorf(
			"%q is a write-only alias that the API never reports back, so Terraform could not track it; "+
				"specify this setting as %q instead",
			key, canonical,
		)
	}
	return nil
}
