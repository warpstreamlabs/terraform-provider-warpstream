package utils

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ basetypes.StringTypable = YamlType{}

type YamlType struct {
	basetypes.StringType
}

func (t YamlType) Equal(o attr.Type) (ok bool) {
	defer func() {
		fmt.Println("YAMLTYPE - Equal", ok)
	}()
	_, ok = o.(YamlType)
	if ok {
		return true
	}
	return t.StringType.Equal(o)
}

func (t YamlType) String() string {
	return "YamlType"
}

func (t YamlType) ValueFromString(ctx context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	// CustomStringValue defined in the value type section
	value := YamlValue{
		StringValue: in,
	}
	return value, nil
}

func (t YamlType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}
	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}
	return stringValuable, nil
}
func (t YamlType) ValueType(ctx context.Context) attr.Value {
	// CustomStringValue defined in the value type section
	return YamlValue{}
}

// CustomStringType defined in the schema type section
func (t YamlType) Validate(ctx context.Context, value tftypes.Value, valuePath path.Path) diag.Diagnostics {
	if value.IsNull() || !value.IsKnown() {
		return nil
	}
	var diags diag.Diagnostics
	var valueString string
	if err := value.As(&valueString); err != nil {
		diags.AddAttributeError(
			valuePath,
			"Invalid YAML String Value",
			"An unexpected error occurred while converting a string value that was expected to be YAML format.\n\n"+
				"Path: "+valuePath.String()+"\n"+
				"Given Value: "+valueString+"\n"+
				"Error: "+err.Error(),
		)
		return diags
	}
	if _, err := NormalizeYAML(valueString); err != nil {
		diags.AddAttributeError(
			valuePath,
			"Invalid YAML String Value",
			"An unexpected error occurred while converting a string value that was expected to be YAML format.\n\n"+
				"Path: "+valuePath.String()+"\n"+
				"Given Value: "+valueString+"\n"+
				"Error: "+err.Error(),
		)
		return diags
	}
	return diags
}

func StringToYaml(value string) YamlValue {
	return YamlValue{
		StringValue: basetypes.NewStringValue(value),
	}
}
