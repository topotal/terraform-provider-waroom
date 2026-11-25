package validator

import (
	"context"
	"strings"
	"unicode"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// IsBlank reports whether s is empty or made only of Unicode whitespace
// (e.g. U+3000 ideographic space, NBSP, \v), not only ASCII whitespace.
func IsBlank(s string) bool {
	return strings.TrimFunc(s, unicode.IsSpace) == ""
}

var _ validator.String = notBlankValidator{}

type notBlankValidator struct{}

func (v notBlankValidator) Description(_ context.Context) string {
	return "must contain a non-whitespace character"
}

func (v notBlankValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v notBlankValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if IsBlank(req.ConfigValue.ValueString()) {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			v.Description(ctx),
			req.ConfigValue.String(),
		))
	}
}

// NotBlank validates that a string contains a non-whitespace character.
// Unlike a `\S` regex, any Unicode whitespace counts as whitespace.
//
// Validators are skipped for values unknown at plan time, so resources that must
// not send a blank value should also check it with IsBlank at apply time.
func NotBlank() validator.String {
	return notBlankValidator{}
}
