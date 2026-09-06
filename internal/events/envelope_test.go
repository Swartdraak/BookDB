package events

import (
	"testing"
	"time"
)

func TestNewEnvelopeUsesVersionAndCopiesPayload(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)
	env := NewEnvelope("BookDB.Events", "Created", payload)
	if env.Version != EnvelopeVersion {
		t.Fatalf("unexpected envelope version %q", env.Version)
	}
	if env.Subject != "bookdb.events" {
		t.Fatalf("unexpected subject %q", env.Subject)
	}
	if env.ContentType != "application/json" {
		t.Fatalf("unexpected content type %q", env.ContentType)
	}
	if env.Timestamp.IsZero() || env.Timestamp.Location() != time.UTC {
		t.Fatalf("unexpected timestamp %v", env.Timestamp)
	}
	payload[0] = '{'
	if string(env.Payload) != `{"hello":"world"}` {
		t.Fatal("payload was not copied")
	}
}

func TestVersionedSubjectAndStreamPattern(t *testing.T) {
	if got := VersionedSubject("BookDB.Events", 1, "Published"); got != "bookdb.events.v1.published" {
		t.Fatalf("unexpected subject %q", got)
	}
	if got := StreamPattern("BookDB.Events", 1); got != "bookdb.events.v1.>" {
		t.Fatalf("unexpected stream pattern %q", got)
	}
}
