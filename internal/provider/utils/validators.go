package utils

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func StartsWith(prefix string) validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(fmt.Sprintf("^%s[a-zA-Z0-9_]+$", prefix)),
		fmt.Sprintf("must start with '%s' and must contain underscores and alphanumeric characters only", prefix),
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
