package provider

import (
	"strings"
	"testing"
)

func TestParseServiceScopedImportID(t *testing.T) {
	const id = "123e4567-e89b-12d3-a456-426614174000"

	serviceName, got, err := parseServiceScopedImportID("my-service," + id)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if serviceName != "my-service" || got != id {
		t.Errorf("got (%q, %q), want (%q, %q)", serviceName, got, "my-service", id)
	}

	// Other UUID spellings are normalized to the canonical form the API returns.
	for _, in := range []string{strings.ToUpper(id), "{" + id + "}", "urn:uuid:" + id} {
		if _, got, err := parseServiceScopedImportID("my-service," + in); err != nil || got != id {
			t.Errorf("parseServiceScopedImportID(%q) = (%q, %v), want (%q, nil)", in, got, err, id)
		}
	}

	for _, in := range []string{"", id, "my-service", "my-service,", "," + id, "my-service,not-a-uuid", "a,b," + id} {
		if _, _, err := parseServiceScopedImportID(in); err == nil {
			t.Errorf("expected an error for %q", in)
		}
	}
}
