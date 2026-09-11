package api

import "github.com/google/uuid"

// parseUUID is a helper to parse a UUID string.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

