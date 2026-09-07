package natsbus

import (
	"context"
	"errors"
	"testing"
	"time"

	nats "github.com/nats-io/nats.go"
)

type fakeJetStream struct {
	infoErr error
	added   *nats.StreamConfig
}

func (f *fakeJetStream) StreamInfo(name string, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	return &nats.StreamInfo{Config: nats.StreamConfig{Name: name}}, nil
}

func (f *fakeJetStream) AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error) {
	f.added = cfg
	return &nats.StreamInfo{Config: *cfg}, nil
}

func TestDevStreamSpecUsesVersionedSubjects(t *testing.T) {
	spec := DevStreamSpec("BOOKDB")
	if len(spec.Subjects) != 2 {
		t.Fatalf("unexpected subject count %d", len(spec.Subjects))
	}
	if spec.Subjects[0] != "bookdb.events.v1.>" {
		t.Fatalf("unexpected event subject %q", spec.Subjects[0])
	}
	if spec.Subjects[1] != "bookdb.commands.v1.>" {
		t.Fatalf("unexpected command subject %q", spec.Subjects[1])
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestEnsureStreamAddsMissingStream(t *testing.T) {
	js := &fakeJetStream{infoErr: nats.ErrStreamNotFound}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := EnsureStream(ctx, js, DevStreamSpec("BOOKDB")); err != nil {
		t.Fatalf("EnsureStream: %v", err)
	}
	if js.added == nil {
		t.Fatal("expected stream to be added")
	}
	if js.added.Name != "BOOKDB" {
		t.Fatalf("unexpected stream name %q", js.added.Name)
	}
	if got := len(js.added.Subjects); got != 2 {
		t.Fatalf("unexpected stream subject count %d", got)
	}
}

func TestEnsureStreamReturnsValidationErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := EnsureStream(ctx, &fakeJetStream{}, StreamSpec{}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestEnsureStreamPassesThroughAdminErrors(t *testing.T) {
	js := &fakeJetStream{infoErr: errors.New("boom")}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := EnsureStream(ctx, js, DevStreamSpec("BOOKDB")); err == nil {
		t.Fatal("expected admin error")
	}
}
