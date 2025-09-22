package validator

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// RFC 4122 UUID (v1–v5), case-insensitive, 8-4-4-4-12
var uuidRegex = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`,
)

// UUID validates RFC4122 UUID (v1–v5).
func UUID() validator.String {
	return stringvalidator.RegexMatches(
		uuidRegex,
		"must be a valid UUID (RFC 4122, 8-4-4-4-12 hex format)",
	)
}
