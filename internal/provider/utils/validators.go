package utils

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func StartsWithAndAlphanumeric(prefix string) validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(fmt.Sprintf("^%s[a-zA-Z0-9_]+$", prefix)),
		fmt.Sprintf("must start with '%s' and must contain underscores and alphanumeric characters only", prefix),
	)
}

func ValidClusterID() validator.String {
	return stringvalidator.All(
		StartsWithAndAlphanumeric("vci_"),
	)
}

func ValidClusterName() validator.String {
	return stringvalidator.All(
		StartsWithAndAlphanumeric("vcn_"),
		stringvalidator.LengthBetween(3, 128),
	)
}

func ValidSchemaRegistryName() validator.String {
	return stringvalidator.All(
		StartsWithAndAlphanumeric("vcn_sr_"),
		stringvalidator.LengthBetween(3, 128),
	)
}

func alphaNumericSpaceesUnderscoresHyphensOnly() validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-z_\-A-Z0-9 ]*$`),
		"must contain only alphanumeric characters, spaces, underscores, and hyphens",
	)
}

func ValidWorkspaceName() validator.String {
	return stringvalidator.All(
		stringvalidator.LengthBetween(3, 128),
		alphaNumericSpaceesUnderscoresHyphensOnly(),
	)
}

func ValidUserRoleName() validator.String {
	return stringvalidator.All(
		stringvalidator.LengthBetween(3, 60),
		alphaNumericSpaceesUnderscoresHyphensOnly(),
	)
}

func StartsWith(prefix string) validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(fmt.Sprintf("^%s.+$", prefix)),
		fmt.Sprintf("must start with '%s'", prefix),
	)
}

// Unfortunately, Golang's regex doesn't support negative lookahead,
// so we can't do ^(?!prefix).
type notStartWithValidator struct {
	prefix string
}

func (validator notStartWithValidator) Description(_ context.Context) string {
	return fmt.Sprintf("value must not start with '%s'", validator.prefix)
}

func (validator notStartWithValidator) MarkdownDescription(ctx context.Context) string {
	return validator.Description(ctx)
}

func (v notStartWithValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	if strings.HasPrefix(value, v.prefix) {
		response.Diagnostics.AddError("invalid prefix", fmt.Sprintf("property must not start with: %s", v.prefix))
	}
}

func NotStartWith(prefix string) validator.String {
	return notStartWithValidator{prefix: prefix}
}
