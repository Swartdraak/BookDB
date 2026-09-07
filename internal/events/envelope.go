// Package events defines BookDB's event-envelope and subject-versioning
// conventions. It stays intentionally small so messaging code can depend on a
// stable wire contract without importing business semantics.
package events

import (
	"fmt"
	"strings"
	"time"
)

// EnvelopeVersion is the canonical event envelope schema version.
const EnvelopeVersion = "v1"

// Envelope is the minimal versioned wrapper used for BookDB events.
type Envelope struct {
	Version       string    `json:"version"`
	ID            string    `json:"id,omitempty"`
	Type          string    `json:"type,omitempty"`
	Subject       string    `json:"subject"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	CausationID   string    `json:"causation_id,omitempty"`
	ContentType   string    `json:"content_type,omitempty"`
	Payload       []byte    `json:"payload,omitempty"`
}

// NewEnvelope creates a v1 event envelope with a copied payload.
func NewEnvelope(subject, typ string, payload []byte) Envelope {
	return Envelope{
		Version:     EnvelopeVersion,
		Type:        strings.TrimSpace(typ),
		Subject:     Subject(subject),
		Timestamp:   time.Now().UTC(),
		ContentType: "application/json",
		Payload:     append([]byte(nil), payload...),
	}
}

// VersionTag returns the canonical token for a schema or subject version.
func VersionTag(version int) string {
	if version < 1 {
		version = 1
	}
	return fmt.Sprintf("v%d", version)
}

// Subject normalizes a dot-delimited subject or token list.
func Subject(parts ...string) string {
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		switch token {
		case ">", "*":
			tokens = append(tokens, token)
		default:
			token = strings.ToLower(token)
			token = strings.ReplaceAll(token, " ", "-")
			token = strings.ReplaceAll(token, "_", "-")
			tokens = append(tokens, token)
		}
	}
	return strings.Join(tokens, ".")
}

// VersionedSubject builds a normalized subject with the version token embedded.
func VersionedSubject(namespace string, version int, suffix ...string) string {
	tokens := make([]string, 0, len(suffix)+2)
	tokens = append(tokens, namespace, VersionTag(version))
	tokens = append(tokens, suffix...)
	return Subject(tokens...)
}

// StreamPattern builds the wildcard subject pattern used by a JetStream stream.
func StreamPattern(namespace string, version int) string {
	return VersionedSubject(namespace, version, ">")
}
