package utils

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"gopkg.in/yaml.v3"
)

var (
	_ basetypes.StringValuable                   = (*YamlValue)(nil)
	_ basetypes.StringValuableWithSemanticEquals = (*YamlValue)(nil)
	_ xattr.ValidateableAttribute                = (*YamlValue)(nil)
	_ function.ValidateableParameter             = (*YamlValue)(nil)
)

type YamlValue struct {
	basetypes.StringValue
}

// Type returns a YamlValueType.
func (v YamlValue) Type(_ context.Context) attr.Type {
	return YamlType{}
}

// Equal returns true if the given value is equivalent.
func (v YamlValue) Equal(o attr.Value) bool {
	other, ok := o.(YamlValue)

	if !ok {
		return false
	}

	return v.StringValue.Equal(other.StringValue)
}

func (v YamlValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(YamlValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", v)+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", newValuable),
		)

		return false, diags
	}

	normalizedValue, _ := NormalizeYAML(v.ValueString())
	normalizedNewValue, _ := NormalizeYAML(newValue.ValueString())

	return normalizedValue == normalizedNewValue, diags
}

func (v YamlValue) ValidateAttribute(ctx context.Context, req xattr.ValidateAttributeRequest, resp *xattr.ValidateAttributeResponse) {
	if v.IsUnknown() || v.IsNull() {
		return
	}

	if _, err := NormalizeYAML(v.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid YAML String Value",
			"A string value was provided that is not valid YAML format.\n\n"+
				"Given Value: "+v.ValueString()+"\n"+
				"Error: "+err.Error()+"\n",
		)

		return
	}
}

func (v YamlValue) ValidateParameter(ctx context.Context, req function.ValidateParameterRequest, resp *function.ValidateParameterResponse) {
	if v.IsUnknown() || v.IsNull() {
		return
	}

	if _, err := NormalizeYAML(v.ValueString()); err != nil {
		resp.Error = function.NewArgumentFuncError(
			req.Position,
			"Invalid YAML String Value: "+
				"A string value was provided that is not valid YAML string format.\n\n"+
				"Given Value: "+v.ValueString()+"\n"+
				"Error: "+err.Error()+"\n",
		)

		return
	}
}

func (v YamlValue) Unmarshal(target any) diag.Diagnostics {
	var diags diag.Diagnostics

	if v.IsNull() {
		diags.Append(diag.NewErrorDiagnostic("YamlValue YAML Unmarshal Error", "yaml string value is null"))
		return diags
	}

	if v.IsUnknown() {
		diags.Append(diag.NewErrorDiagnostic("YamlValue YAML Unmarshal Error", "yaml string value is unknown"))
		return diags
	}

	err := yaml.Unmarshal([]byte(v.ValueString()), target)
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("YamlValue YAML Unmarshal Error", err.Error()))
	}

	return diags
}

func NormalizeYAML(input string) (string, error) {
	var data interface{}
	err := yaml.Unmarshal([]byte(input), &data)
	if err != nil {
		return "", err
	}

	normalized, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(normalized), nil
}
