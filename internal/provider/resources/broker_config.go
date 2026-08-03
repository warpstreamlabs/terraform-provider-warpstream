package resources

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

// typedAttrConfig is a cluster broker config that also has a typed `configuration` attribute.
type typedAttrConfig struct {
	// key is the Kafka-style config name the API knows the setting by.
	key string
	// typedAttr is the `configuration` attribute that controls it.
	typedAttr string
	// planValue reads that attribute out of a planned `configuration`.
	planValue func(models.VirtualClusterConfiguration) attr.Value
}

var typedAttrConfigs = []typedAttrConfig{
	{"auto.create.topics.enable", "auto_create_topic",
		func(c models.VirtualClusterConfiguration) attr.Value { return c.AutoCreateTopic }},
	{"num.partitions", "default_num_partitions",
		func(c models.VirtualClusterConfiguration) attr.Value { return c.DefaultNumPartitions }},
	{"log.retention.ms", "default_retention_millis",
		func(c models.VirtualClusterConfiguration) attr.Value { return c.DefaultRetention }},
	{"warpstream.default.topic.type", "default_topic_type",
		func(c models.VirtualClusterConfiguration) attr.Value { return c.DefaultTopicType }},
	{"warpstream.soft.delete.topic.enable", "enable_soft_topic_deletion",
		func(c models.VirtualClusterConfiguration) attr.Value { return c.EnableSoftTopicDeletion }},
	{"warpstream.soft.delete.topic.ttl.ms", "soft_topic_deletion_ttl_millis",
		func(c models.VirtualClusterConfiguration) attr.Value { return c.SoftTopicDeletionTTL }},
}

// typedAttrFor returns the `configuration` attribute that owns a config name.
func typedAttrFor(key string) (string, bool) {
	for _, m := range typedAttrConfigs {
		if m.key == key {
			return m.typedAttr, true
		}
	}
	return "", false
}

// renderConfigValue spells a planned typed attribute the way broker_configs does, and reports
// false when the plan does not carry a value for it yet.
func renderConfigValue(v attr.Value) (string, bool) {
	if v == nil || v.IsNull() || v.IsUnknown() {
		return "", false
	}
	switch t := v.(type) {
	case types.Bool:
		return strconv.FormatBool(t.ValueBool()), true
	case types.Int64:
		return strconv.FormatInt(t.ValueInt64(), 10), true
	case types.String:
		return t.ValueString(), true
	default:
		return "", false
	}
}

// WriteOnlyAliasKeys names configs the API accepts on write but never reports back on describe,
// mapped to the canonical name each is an alternate unit for. A value declared under one of these
// could never be read back, so Terraform would fail comparing state against the configuration —
// which is why they are rejected rather than translated.
var WriteOnlyAliasKeys = map[string]string{
	"log.retention.minutes":                  "log.retention.ms",
	"log.retention.hours":                    "log.retention.ms",
	"warpstream.soft.delete.topic.ttl.hours": "warpstream.soft.delete.topic.ttl.ms",
}

// validateBrokerConfigKey rejects the config names `broker_configuration` cannot hold. Unknown names are
// deliberately not rejected: the API is the authority on which configs exist.
func validateBrokerConfigKey(key string) error {
	canonical, isAlias := WriteOnlyAliasKeys[key]
	if !isAlias {
		canonical = key
	}
	typedAttr, hasTypedAttr := typedAttrFor(canonical)

	var problem string
	switch {
	case isAlias:
		problem = fmt.Sprintf("%q is an alternate unit for %q, which the API never reports back, "+
			"so Terraform could not track it", key, canonical)
	case hasTypedAttr:
		problem = fmt.Sprintf("%q is controlled by the `configuration.%s` attribute in this provider",
			key, typedAttr)
	default:
		return nil
	}

	if hasTypedAttr {
		return fmt.Errorf("%s; set `configuration.%s` instead", problem, typedAttr)
	}
	return fmt.Errorf("%s; specify this setting as %q instead", problem, canonical)
}
