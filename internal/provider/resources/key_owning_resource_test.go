package resources

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestKeyOwningResourceStripDoesNotChangeItsInput(t *testing.T) {
	ctx := context.Background()
	r := &keyOwningResource{kind: agentKeyKind}
	base := schema.Schema{Attributes: map[string]schema.Attribute{"id": schema.StringAttribute{Computed: true}}}
	keyType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}}
	fullType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String, "agent_key": keyType}}
	full := tftypes.NewValue(fullType, map[string]tftypes.Value{
		"id":        tftypes.NewValue(tftypes.String, "vci_1"),
		"agent_key": tftypes.NewValue(keyType, map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "aki_1")}),
	})

	stripped, diags := r.strip(ctx, base, full)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !stripped.Type().Equal(base.Type().TerraformType(ctx)) {
		t.Fatalf("unexpected type %s", stripped.Type())
	}

	// The framework reuses the plan after Create, so stripping must leave the key in it.
	var attrs map[string]tftypes.Value
	if err := full.As(&attrs); err != nil {
		t.Fatal(err)
	}
	if _, ok := attrs["agent_key"]; !ok {
		t.Fatal("strip removed the key from its input")
	}
	_ = full.Copy()

	back, err := withAttribute(fullType, stripped, "agent_key", attrs["agent_key"])
	if err != nil {
		t.Fatal(err)
	}
	if !back.Equal(full) {
		t.Fatalf("round trip changed the value: %s", back)
	}
}

func TestOwnedKeyName(t *testing.T) {
	name := ownedKeyName("My Workspace-1", "application_key")
	if !regexp.MustCompile(`^akn_my_workspace_1_application_key_[0-9a-f]{6}$`).MatchString(name) {
		t.Fatalf("unexpected name %q", name)
	}
}
