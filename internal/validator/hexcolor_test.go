package validator_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	v "github.com/topotal/terraform-provider-waroom/internal/validator"
)

func runHex6(t *testing.T, v validator.String, raw string, wantOK bool) {
	t.Helper()
	req := validator.StringRequest{
		ConfigValue: types.StringValue(raw),
	}
	var resp validator.StringResponse
	v.ValidateString(context.Background(), req, &resp)
	if wantOK && resp.Diagnostics.HasError() {
		t.Fatalf("hex OK expected for %q, got errors: %v", raw, resp.Diagnostics)
	}
	if !wantOK && !resp.Diagnostics.HasError() {
		t.Fatalf("hex error expected for %q, got OK", raw)
	}
}

func TestHex6NoHash(t *testing.T) {
	val := v.Hex6NoHash()
	ok := []string{"ffffff", "ABCDEF", "00ffcc", "112233"}
	ng := []string{"#ffffff", "fffff", "gggggg", "11223344", "🤮", ""}
	for _, s := range ok {
		runHex6(t, val, s, true)
	}
	for _, s := range ng {
		runHex6(t, val, s, false)
	}
}
