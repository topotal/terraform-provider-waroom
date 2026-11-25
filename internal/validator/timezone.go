package validator

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/topotal/waroom-client-go/waroom"
)

var _ validator.String = timezoneValidator{}

type timezoneValidator struct{}

func (v timezoneValidator) Description(_ context.Context) string {
	return "must be an IANA time zone name (e.g. Asia/Tokyo)"
}

func (v timezoneValidator) MarkdownDescription(_ context.Context) string {
	return "must be an IANA time zone name (e.g. `Asia/Tokyo`)"
}

func (v timezoneValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if err := waroom.ValidateTimezone(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			v.Description(ctx),
			req.ConfigValue.String(),
		))
	}
}

// Timezone validates an IANA time zone name with waroom.ValidateTimezone.
// Aliases such as Japan and Etc/UTC are accepted; the API stores them as sent.
//
// The check is best effort: a name with the wrong case (e.g. asia/tokyo) passes
// on case-insensitive file systems such as macOS, and is rejected by the API at apply.
func Timezone() validator.String {
	return timezoneValidator{}
}
