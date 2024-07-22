package utils

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestCheckResourceAttrStartsWith(pathToAttr, attr, prefix string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(
		pathToAttr,
		attr,
		func(value string) error {
			if !strings.HasPrefix(value, prefix) {
				return fmt.Errorf("expected %s to start with '%s', got: %s", attr, prefix, value)
			}
			return nil
		},
	)
}

func TestCheckResourceAttrEndsWith(pathToAttr, attr, suffix string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(
		pathToAttr,
		attr,
		func(value string) error {
			if !strings.HasSuffix(value, suffix) {
				return fmt.Errorf("expected %s to end with '%s', got: %s", attr, suffix, value)
			}
			return nil
		},
	)
}
