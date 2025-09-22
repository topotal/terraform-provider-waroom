package validator

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func Hex6NoHash() validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(`^[0-9a-fA-F]{6}$`),
		"must be a 6-digit hex color code without #",
	)
}
