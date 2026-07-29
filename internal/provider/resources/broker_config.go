package resources

import "fmt"

// typedAttrBrokerKey maps each deprecated `configuration` attribute to the cluster broker
// config name that controls the same underlying setting.
var typedAttrBrokerKey = map[string]string{
	"auto_create_topic":              "auto.create.topics.enable",
	"default_num_partitions":         "num.partitions",
	"default_retention_millis":       "log.retention.ms",
	"default_topic_type":             "warpstream.default.topic.type",
	"enable_soft_topic_deletion":     "warpstream.soft.delete.topic.enable",
	"soft_topic_deletion_ttl_millis": "warpstream.soft.delete.topic.ttl.ms",
}

// brokerKeyTypedAttr is the reverse of typedAttrBrokerKey.
var brokerKeyTypedAttr = func() map[string]string {
	out := make(map[string]string, len(typedAttrBrokerKey))
	for attrName, key := range typedAttrBrokerKey {
		out[key] = attrName
	}
	return out
}()

// writeOnlyAliasKeys names configs the API accepts on write but never reports back on describe,
// because they are alternate units for a canonical key. A value declared under one of these
// could never be read back, so Terraform would fail comparing state against the configuration.
//
// This list is advisory only: it exists to turn a confusing failure into a message naming the
// key to use instead. A config absent from it is still sent to the API, which remains the
// authority on what it accepts. Being incomplete here costs a worse error message, never a
// rejected valid config.
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
