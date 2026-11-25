package validator_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	v "github.com/topotal/terraform-provider-waroom/internal/validator"
)

func runNotBlank(t *testing.T, value types.String, wantOK bool) {
	t.Helper()
	req := validator.StringRequest{
		ConfigValue: value,
	}
	var resp validator.StringResponse
	v.NotBlank().ValidateString(context.Background(), req, &resp)
	if wantOK && resp.Diagnostics.HasError() {
		t.Fatalf("NotBlank OK expected for %s, got errors: %v", value, resp.Diagnostics)
	}
	if !wantOK && !resp.Diagnostics.HasError() {
		t.Fatalf("NotBlank error expected for %s, got OK", value)
	}
}

func TestNotBlank_Valid(t *testing.T) {
	ok := []string{
		"a",
		" a ",
		"\u3000テンプレート\u3000",
		"## Impact\n",
		"\u200b", // zero-width space is not whitespace in Unicode
	}
	for _, s := range ok {
		runNotBlank(t, types.StringValue(s), true)
	}
	runNotBlank(t, types.StringNull(), true)
	runNotBlank(t, types.StringUnknown(), true)
}

func TestNotBlank_Invalid(t *testing.T) {
	ng := []string{
		"",
		" ",
		" \n\t\r",
		"\v\f",
		"\u3000",       // ideographic (full-width) space
		"\u00a0",       // NBSP
		"\u2003\u2028", // em space, line separator
		"\u0085",       // next line
	}
	for _, s := range ng {
		runNotBlank(t, types.StringValue(s), false)
	}
}

func TestIsBlank(t *testing.T) {
	cases := map[string]bool{
		"":             true,
		" \n":          true,
		"\u3000\u00a0": true,
		"x":            false,
		"\u3000x":      false,
	}
	for s, want := range cases {
		if got := v.IsBlank(s); got != want {
			t.Errorf("IsBlank(%q) = %v, want %v", s, got, want)
		}
	}
}
