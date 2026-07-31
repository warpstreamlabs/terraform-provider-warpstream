package resources

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

// mirroredConfig is a cluster broker config that also has a typed `configuration` attribute.
type mirroredConfig struct {
	// key is the Kafka-style config name the API knows the setting by.
	key string
	// typedAttr is the `configuration` attribute that controls it.
	typedAttr string
	// planValue reads that attribute out of a planned `configuration`.
	planValue func(models.VirtualClusterConfiguration) attr.Value
}

// mirroredConfigs is the one place these config names are spelled, so key validation and the
// update payload cannot drift apart.
//
// The two surfaces are disjoint: a setting listed here can only be written through its typed
// attribute, and `broker_configuration` rejects its key at plan time. This set is closed by
// policy — new cluster configs are map-only and must not grow it — so the typed attributes can
// be removed wholesale in the next major version, at which point these keys become legal in
// the map.
var mirroredConfigs = []mirroredConfig{
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
	for _, m := range mirroredConfigs {
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
	canonical, isAlias := writeOnlyAliasKeys[key]
	if !isAlias {
		canonical = key
	}
	typedAttr, isMirrored := typedAttrFor(canonical)

	var problem string
	switch {
	case isAlias:
		problem = fmt.Sprintf("%q is an alternate unit for %q, which the API never reports back, "+
			"so Terraform could not track it", key, canonical)
	case isMirrored:
		problem = fmt.Sprintf("%q is controlled by the `configuration.%s` attribute in this provider",
			key, typedAttr)
	default:
		return nil
	}

	// An alias for a mirrored setting redirects to the typed attribute, as the canonical key
	// itself would; anything else redirects to the canonical spelling.
	if isMirrored {
		return fmt.Errorf("%s; set `configuration.%s` instead", problem, typedAttr)
	}
	return fmt.Errorf("%s; specify this setting as %q instead", problem, canonical)
}
