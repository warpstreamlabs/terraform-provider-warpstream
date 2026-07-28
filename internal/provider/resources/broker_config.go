package resources

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// brokerConfigKind is the value type of a cluster broker config. It determines both how a
// declared value is checked for canonical form and which Terraform type the mirroring typed
// `configuration` attribute uses.
type brokerConfigKind int

const (
	brokerConfigBool brokerConfigKind = iota
	brokerConfigInt
	brokerConfigEnum
)

// brokerConfigSpec describes one entry of the WarpStream cluster broker-config surface.
type brokerConfigSpec struct {
	Kind brokerConfigKind

	// AliasOf, when set, names the canonical key that this key is a write-only alias for.
	// The API accepts aliases on write, but describe only ever emits the canonical name, so
	// a value declared under an alias could never round-trip. We reject aliases outright and
	// tell the user which key to use instead.
	AliasOf string

	// TypedAttr, when set, is the tfsdk name of the deprecated `configuration` attribute
	// that mirrors this config. Both surfaces write the same underlying cluster setting.
	TypedAttr string

	// NegativeIsInfinite marks configs where the API collapses every negative value to "-1"
	// on write, which makes "-1" the only negative value that round-trips.
	NegativeIsInfinite bool

	// EnumValues lists the accepted values for brokerConfigEnum, in canonical form.
	EnumValues []string
}

// brokerConfigs is the set of cluster broker configs the WarpStream API supports in the
// generic `broker_configs` map. Any name absent from this table is rejected by the API on
// write, so we reject it at plan time instead to fail before an apply is half-done.
var brokerConfigs = map[string]brokerConfigSpec{
	// Booleans.
	"auto.create.topics.enable":                         {Kind: brokerConfigBool, TypedAttr: "auto_create_topic"},
	"delete.topic.enable":                               {Kind: brokerConfigBool},
	"warpstream.soft.delete.topic.enable":               {Kind: brokerConfigBool, TypedAttr: "enable_soft_topic_deletion"},
	"warpstream.default.partitions_auto_scaler.enabled": {Kind: brokerConfigBool},

	// Integers.
	"num.partitions":                           {Kind: brokerConfigInt, TypedAttr: "default_num_partitions"},
	"message.max.bytes":                        {Kind: brokerConfigInt},
	"offsets.retention.minutes":                {Kind: brokerConfigInt},
	"group.consumer.max.size":                  {Kind: brokerConfigInt},
	"group.consumer.session.timeout.ms":        {Kind: brokerConfigInt},
	"group.consumer.min.session.timeout.ms":    {Kind: brokerConfigInt},
	"group.consumer.max.session.timeout.ms":    {Kind: brokerConfigInt},
	"group.consumer.heartbeat.interval.ms":     {Kind: brokerConfigInt},
	"group.consumer.min.heartbeat.interval.ms": {Kind: brokerConfigInt},
	"group.consumer.max.heartbeat.interval.ms": {Kind: brokerConfigInt},
	"warpstream.default.partitions_auto_scaler.per_partition_throughput_uncompressed_bytes_per_second": {Kind: brokerConfigInt},
	"warpstream.default.partitions_auto_scaler.max_partition_count":                                    {Kind: brokerConfigInt},

	// Default topic retention. Three names are views of one stored value; only the
	// millisecond name is ever returned by describe.
	"log.retention.ms":      {Kind: brokerConfigInt, TypedAttr: "default_retention_millis", NegativeIsInfinite: true},
	"log.retention.minutes": {AliasOf: "log.retention.ms"},
	"log.retention.hours":   {AliasOf: "log.retention.ms"},

	// Soft-delete topic TTL. Two names are views of one stored value; only the millisecond
	// name is ever returned by describe.
	"warpstream.soft.delete.topic.ttl.ms":    {Kind: brokerConfigInt, TypedAttr: "soft_topic_deletion_ttl_millis", NegativeIsInfinite: true},
	"warpstream.soft.delete.topic.ttl.hours": {AliasOf: "warpstream.soft.delete.topic.ttl.ms"},

	// Enum.
	"warpstream.default.topic.type": {Kind: brokerConfigEnum, TypedAttr: "default_topic_type", EnumValues: []string{"classic", "lightning"}},
}

// typedAttrBrokerKey maps a typed `configuration` attribute's tfsdk name to the canonical
// broker config key that controls the same setting. It is derived from brokerConfigs so the
// two can never disagree.
var typedAttrBrokerKey = func() map[string]string {
	out := make(map[string]string)
	for key, spec := range brokerConfigs {
		if spec.TypedAttr == "" || spec.AliasOf != "" {
			continue
		}
		out[spec.TypedAttr] = key
	}
	return out
}()

// validateBrokerConfigKey reports whether key is a broker config that may be declared in
// `broker_configuration`. Unsupported names and write-only aliases are rejected.
func validateBrokerConfigKey(key string) error {
	spec, ok := brokerConfigs[key]
	if !ok {
		return fmt.Errorf(
			"%q is not a supported cluster broker config; see the WarpStream cluster configuration reference for the supported names",
			key,
		)
	}
	if spec.AliasOf != "" {
		return fmt.Errorf(
			"%q is a write-only alias that the API never reports back, so Terraform could not track it; specify this setting as %q instead",
			key, spec.AliasOf,
		)
	}
	return nil
}

// validateBrokerConfigValue reports whether raw is already in the canonical form the API
// returns on describe. It must be, because `broker_configuration` is not a Computed
// attribute: Terraform requires the value in state after an apply to equal the value in the
// configuration, and state is populated from the API's response.
//
// Range checks (for example the minimum retention, or a non-negative message.max.bytes) are
// deliberately left to the API, which rejects them with a clear error and is the only
// authority that will not drift.
func validateBrokerConfigValue(key, raw string) error {
	spec, ok := brokerConfigs[key]
	if !ok {
		return nil // Unsupported keys are reported by validateBrokerConfigKey.
	}

	canonical, err := canonicalBrokerConfigValue(spec, raw)
	if err != nil {
		return err
	}
	if canonical != raw {
		return fmt.Errorf(
			"write this value as %q, not %q; the API stores and reports the canonical form, and Terraform would fail the apply comparing it against your configuration",
			canonical, raw,
		)
	}
	return nil
}

// canonicalBrokerConfigValue returns the form the API will report back for raw, or an error
// if raw cannot be parsed as the config's type at all.
func canonicalBrokerConfigValue(spec brokerConfigSpec, raw string) (string, error) {
	switch spec.Kind {
	case brokerConfigBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return "", fmt.Errorf("%q is not a boolean; use \"true\" or \"false\"", raw)
		}
		return strconv.FormatBool(b), nil

	case brokerConfigInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return "", fmt.Errorf("%q is not an integer", raw)
		}
		if spec.NegativeIsInfinite && n < 0 {
			// The API collapses any negative value to -1, meaning infinite.
			return "-1", nil
		}
		return strconv.FormatInt(n, 10), nil

	case brokerConfigEnum:
		canonical := strings.ToLower(strings.TrimSpace(raw))
		for _, v := range spec.EnumValues {
			if canonical == v {
				return canonical, nil
			}
		}
		return "", fmt.Errorf("%q is not valid; must be one of %s", raw, strings.Join(quoted(spec.EnumValues), ", "))
	}
	return raw, nil
}

// brokerConfigTypedValue converts a canonical broker config value into the Terraform value
// for the typed `configuration` attribute that mirrors it. The kind in brokerConfigs and the
// typed attribute's schema type are kept in step, so no type inspection is needed here.
func brokerConfigTypedValue(key, raw string) (attr.Value, error) {
	spec, ok := brokerConfigs[key]
	if !ok {
		return nil, fmt.Errorf("%q is not a supported cluster broker config", key)
	}

	switch spec.Kind {
	case brokerConfigBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("%q is not a boolean", raw)
		}
		return types.BoolValue(b), nil
	case brokerConfigInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", raw)
		}
		return types.Int64Value(n), nil
	case brokerConfigEnum:
		return types.StringValue(raw), nil
	}
	return nil, fmt.Errorf("cluster broker config %q has no typed representation", key)
}

// brokerConfigUnknownTypedValue returns the unknown Terraform value of the type used by the
// typed `configuration` attribute mirroring key. It is used when the declared broker config
// value is not known until apply, so the mirroring attribute plans as known-after-apply
// rather than keeping a schema default the apply is about to contradict.
func brokerConfigUnknownTypedValue(key string) (attr.Value, error) {
	spec, ok := brokerConfigs[key]
	if !ok {
		return nil, fmt.Errorf("%q is not a supported cluster broker config", key)
	}

	switch spec.Kind {
	case brokerConfigBool:
		return types.BoolUnknown(), nil
	case brokerConfigInt:
		return types.Int64Unknown(), nil
	case brokerConfigEnum:
		return types.StringUnknown(), nil
	}
	return nil, fmt.Errorf("cluster broker config %q has no typed representation", key)
}

// quoted returns values with each element wrapped in double quotes, for error messages.
func quoted(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = fmt.Sprintf("%q", v)
	}
	return out
}
