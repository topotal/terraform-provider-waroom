package validator_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	v "github.com/topotal/terraform-provider-waroom/internal/validator"
)

func runUUID(t *testing.T, v validator.String, raw string, wantOK bool) {
	t.Helper()
	req := validator.StringRequest{
		ConfigValue: types.StringValue(raw),
	}
	var resp validator.StringResponse
	v.ValidateString(context.Background(), req, &resp)
	if wantOK && resp.Diagnostics.HasError() {
		t.Fatalf("UUID OK expected for %q, got errors: %v", raw, resp.Diagnostics)
	}
	if !wantOK && !resp.Diagnostics.HasError() {
		t.Fatalf("UUID error expected for %q, got OK", raw)
	}
}

func TestUUID_Valid(t *testing.T) {
	v := v.UUID()
	ok := []string{
		"550e8400-e29b-41d4-a716-446655440000", // v4
		"f47ac10b-58cc-4372-a567-0e02b2c3d479", // v4
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8", // v1
		"6BA7B811-9DAD-11D1-80B4-00C04FD430C8", // upper case
		"01890a5d-ac96-774b-bcce-b302099a8057", // v7 (RFC 9562)
		"550e8400-e29b-41d4-c716-446655440000", // non-RFC 4122 variant
		"00000000-0000-0000-0000-000000000000", // nil UUID
		"ffffffff-ffff-ffff-ffff-ffffffffffff", // max UUID
	}
	for _, s := range ok {
		runUUID(t, v, s, true)
	}
}

func TestUUID_Invalid(t *testing.T) {
	v := v.UUID()
	ng := []string{
		"", "😀", "foobar", "550e8400e29b41d4a716446655440000", // no hyphens
		"zz0e8400-e29b-41d4-a716-446655440000",
		"550e8400-e29b-41d4-a716-44665544000",           // too short
		"{550e8400-e29b-41d4-a716-446655440000}",        // braces
		"urn:uuid:550e8400-e29b-41d4-a716-446655440000", // URN
	}
	for _, s := range ng {
		runUUID(t, v, s, false)
	}
}
