package resources

import "fmt"

// brokerKeyTypedAttr maps a cluster broker config name to the deprecated `configuration`
// attribute that controls the same underlying setting.
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
var writeOnlyAliasKeys = map[string]string{
	"log.retention.minutes":                  "log.retention.ms",
	"log.retention.hours":                    "log.retention.ms",
	"warpstream.soft.delete.topic.ttl.hours": "warpstream.soft.delete.topic.ttl.ms",
}

// validateBrokerConfigKey rejects the config names Terraform could never track. Unknown names
// are deliberately not rejected: the API is the authority on which configs exist, and adding
// one there must not require a provider release.
func validateBrokerConfigKey(key string) error {
	if canonical, ok := writeOnlyAliasKeys[key]; ok {
		return fmt.Errorf(
			"%q is a write-only alias that the API never reports back, so Terraform could not track it; "+
				"specify this setting as %q instead",
			key, canonical,
		)
	}
	return nil
}
