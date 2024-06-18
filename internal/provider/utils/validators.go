package utils

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func StartsWith(prefix string) validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(fmt.Sprintf("^%s[a-zA-Z0-9_]+$", prefix)),
		fmt.Sprintf("must start with '%s' and must contain underscores and alphanumeric characters only", prefix),
	)
}
