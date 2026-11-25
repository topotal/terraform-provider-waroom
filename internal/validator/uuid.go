package validator

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// UUID in the 8-4-4-4-12 hex form, case-insensitive. Any version and variant is accepted
// (e.g. v7 from RFC 9562, the nil and max UUIDs); other spellings that uuid.Parse accepts
// ({...}, urn:uuid:..., no hyphens) are rejected because the value is used as is in API paths.
var uuidRegex = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
)

// UUID validates a UUID in the 8-4-4-4-12 hex form, of any version.
func UUID() validator.String {
	return stringvalidator.RegexMatches(
		uuidRegex,
		"must be a valid UUID (8-4-4-4-12 hex format)",
	)
}
