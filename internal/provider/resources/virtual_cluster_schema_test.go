package resources

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// TestVirtualClusterSchemaComputedNilAttrsSettle guards against unresolvable diffs that arise
// when a Computed attribute with no Default is null in state.
//
// For such attributes, Terraform marks them known-after-apply whenever the resource differs from prior state for any reason.
// On apply, if the attribute continues to be null in state, the diff will always stay. This is usually not an issue,
// however, when using something like ModifyPlan, Terraform marks the attribute as unknown in the plan before ModifyPlan runs.
// Thus if after ModifyPlan runs, there is no diff between the modified plan and the prior state, Terraform will continue to
// mark the attribute as known-after-apply, and the diff will never settle.
//
// This test ensures that all such attributes are either 1) allowed to be null in state, and thus have the UseStateForUnknownIncludingNull modifier attached,
// or 2) are always returned by the API and thus will never be null in state.
func TestVirtualClusterSchemaComputedNilAttrsSettle(t *testing.T) {
	t.Parallel()

	// Attributes that are Computed with no Default but can never be null in state after an
	// apply, because the provider always writes an API-provided value for them.
	allowlist := map[string]string{
		"id":              "always returned by create/describe",
		"agent_pool_id":   "always returned by describe",
		"agent_pool_name": "always returned by describe",
		"created_at":      "always returned by describe",
		"default":         "computed from the name on every read",
		"bootstrap_url":   "always returned by describe for BYOC clusters",
		"workspace_id":    "always returned by describe",
		"tags":            "readTags always writes a (possibly empty) map",
		// Elements of event_types only exist when the parent map does, and the parent map is
		// repaired by UseStateForUnknownIncludingNullMap; the API returns both fields for
		// every event type it reports.
		"events.event_types[*].enabled":                "returned for every event type",
		"events.event_types[*].retention_period_nanos": "returned for every event type",
	}

	var resp resource.SchemaResponse
	(&virtualClusterResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}

	seen := make(map[string]bool)
	walkAttributes(t, resp.Schema.Attributes, "", func(path string, a schema.Attribute) {
		computed, hasDefault, includingNull := attrPlanFacts(t, path, a)
		if !computed || hasDefault {
			return
		}
		if includingNull {
			return
		}
		if _, ok := allowlist[path]; ok {
			seen[path] = true
			return
		}
		t.Errorf(
			"%s is Computed with no Default and no UseStateForUnknownIncludingNull modifier: "+
				"if its value can ever be null in state, every plan after that will report a "+
				"phantom change. Attach the modifier, or allowlist it with a justification.",
			path,
		)
	})

	// A stale allowlist entry means the attribute changed shape; make someone look.
	for path := range allowlist {
		if !seen[path] {
			t.Errorf("allowlist entry %q no longer matches a Computed no-Default attribute; remove it", path)
		}
	}
}

// walkAttributes visits every attribute in the schema, including those nested inside
// single-nested and map-nested attributes. Paths use dotted notation, with "[*]" standing for
// map elements.
func walkAttributes(t *testing.T, attrs map[string]schema.Attribute, prefix string, visit func(path string, a schema.Attribute)) {
	t.Helper()
	for name, a := range attrs {
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		visit(path, a)
		switch v := a.(type) {
		case schema.SingleNestedAttribute:
			walkAttributes(t, v.Attributes, path, visit)
		case schema.MapNestedAttribute:
			walkAttributes(t, v.NestedObject.Attributes, path+"[*]", visit)
		case schema.ListNestedAttribute:
			walkAttributes(t, v.NestedObject.Attributes, path+"[*]", visit)
		case schema.SetNestedAttribute:
			walkAttributes(t, v.NestedObject.Attributes, path+"[*]", visit)
		}
	}
}

// attrPlanFacts extracts, for any concrete attribute type this schema uses, whether it is
// Computed, whether it has a Default, and whether its plan modifiers include one of the
// utils.UseStateForUnknownIncludingNull variants (matched by type, not description).
func attrPlanFacts(t *testing.T, path string, a schema.Attribute) (computed, hasDefault, includingNull bool) {
	t.Helper()

	is := func(mod any) bool {
		// All variants are the same unexported empty struct, so comparing against any
		// constructor's value matches every variant of the modifier.
		return mod == any(utils.UseStateForUnknownIncludingNullString())
	}

	switch v := a.(type) {
	case schema.StringAttribute:
		for _, m := range v.PlanModifiers {
			includingNull = includingNull || is(m)
		}
		return v.Computed, v.Default != nil, includingNull
	case schema.BoolAttribute:
		for _, m := range v.PlanModifiers {
			includingNull = includingNull || is(m)
		}
		return v.Computed, v.Default != nil, includingNull
	case schema.Int64Attribute:
		for _, m := range v.PlanModifiers {
			includingNull = includingNull || is(m)
		}
		return v.Computed, v.Default != nil, includingNull
	case schema.MapAttribute:
		for _, m := range v.PlanModifiers {
			includingNull = includingNull || is(m)
		}
		return v.Computed, v.Default != nil, includingNull
	case schema.SingleNestedAttribute:
		for _, m := range v.PlanModifiers {
			includingNull = includingNull || is(m)
		}
		return v.Computed, v.Default != nil, includingNull
	case schema.MapNestedAttribute:
		for _, m := range v.PlanModifiers {
			includingNull = includingNull || is(m)
		}
		return v.Computed, v.Default != nil, includingNull
	case schema.ListNestedAttribute:
		for _, m := range v.PlanModifiers {
			includingNull = includingNull || is(m)
		}
		return v.Computed, v.Default != nil, includingNull
	default:
		t.Fatalf("%s: unhandled attribute type %s — extend attrPlanFacts", path, fmt.Sprintf("%T", a))
		return false, false, false
	}
}
