package provider

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// parseServiceScopedImportID splits the import ID of a resource that lives under a service,
// in the form "service_name,uuid". The UUID is returned in canonical form (lowercase, hyphenated),
// as the API returns it, so it matches the API's value in Read.
func parseServiceScopedImportID(importID string) (string, string, error) {
	parts := strings.Split(importID, ",")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import ID format: service_name,uuid. Got: %s", importID)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("the UUID part of the import ID is not valid: %s", err)
	}
	return parts[0], id.String(), nil
}

// parseUUIDImportID checks the import ID of a top-level resource, which is its UUID, and returns the
// UUID in canonical form (lowercase, hyphenated), as the API returns it, so it matches the API's value in Read.
func parseUUIDImportID(importID string) (string, error) {
	id, err := uuid.Parse(importID)
	if err != nil {
		return "", fmt.Errorf("expected import ID format: uuid. Got: %s (%s)", importID, err)
	}
	return id.String(), nil
}
