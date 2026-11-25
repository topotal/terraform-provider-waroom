package validator_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	v "github.com/topotal/terraform-provider-waroom/internal/validator"
)

func runTimezone(t *testing.T, value types.String, wantOK bool) {
	t.Helper()
	req := validator.StringRequest{
		ConfigValue: value,
	}
	var resp validator.StringResponse
	v.Timezone().ValidateString(context.Background(), req, &resp)
	if wantOK && resp.Diagnostics.HasError() {
		t.Fatalf("Timezone OK expected for %s, got errors: %v", value, resp.Diagnostics)
	}
	if !wantOK && !resp.Diagnostics.HasError() {
		t.Fatalf("Timezone error expected for %s, got OK", value)
	}
}

func TestTimezone_Valid(t *testing.T) {
	ok := []string{
		"Asia/Tokyo",
		"America/New_York",
		"UTC",
		"Etc/UTC",
		"Japan", // alias
	}
	for _, s := range ok {
		runTimezone(t, types.StringValue(s), true)
	}
	runTimezone(t, types.StringNull(), true)
	runTimezone(t, types.StringUnknown(), true)
}

func TestTimezone_Invalid(t *testing.T) {
	ng := []string{
		"",
		"Local",
		"Tokyo",
		"Asia/Tokyo ",
		"+09:00",
	}
	for _, s := range ng {
		runTimezone(t, types.StringValue(s), false)
	}
}
