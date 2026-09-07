// Package natsbus provides the NATS JetStream foundation BookDB uses for
// durable work and event publication.
package natsbus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bookdb/bookdb/internal/events"
	nats "github.com/nats-io/nats.go"
)

// Client wraps a NATS connection and exposes the small set of operations BookDB
// needs at the foundation layer.
type Client struct {
	conn *nats.Conn
}

// Connect establishes a NATS connection and flushes it before returning.
func Connect(ctx context.Context, url string, opts ...nats.Option) (*Client, error) {
	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, err
	}
	client := &Client{conn: conn}
	if err := client.Check(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return client, nil
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.conn.Close()
	return nil
}

// Check flushes the connection to prove the NATS endpoint is reachable.
func (c *Client) Check(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("nats: client is nil")
	}
	return c.conn.FlushWithContext(ctx)
}

// JetStream returns the JetStream admin context.
func (c *Client) JetStream() (JetStreamAdmin, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("nats: client is nil")
	}
	js, err := c.conn.JetStream()
	if err != nil {
		return nil, err
	}
	return js, nil
}

// JetStreamAdmin is the narrow admin surface needed for bootstrap.
type JetStreamAdmin interface {
	StreamInfo(name string, opts ...nats.JSOpt) (*nats.StreamInfo, error)
	AddStream(cfg *nats.StreamConfig, opts ...nats.JSOpt) (*nats.StreamInfo, error)
}

// StreamSpec describes a stream bootstrap configuration.
type StreamSpec struct {
	Name        string
	Description string
	Subjects    []string
	Retention   nats.RetentionPolicy
	Storage     nats.StorageType
	Discard     nats.DiscardPolicy
	MaxAge      time.Duration
}

// Validate checks the stream specification before it reaches the server.
func (s StreamSpec) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("nats: stream name is required")
	}
	if len(s.Subjects) == 0 {
		return fmt.Errorf("nats: at least one subject is required")
	}
	return nil
}

// DevStreamSpec returns the development stream used by the compose stack.
func DevStreamSpec(streamName string) StreamSpec {
	return StreamSpec{
		Name:        streamName,
		Description: "BookDB development event stream",
		Subjects: []string{
			events.StreamPattern("bookdb.events", 1),
			events.StreamPattern("bookdb.commands", 1),
		},
		Retention: nats.LimitsPolicy,
		Storage:   nats.FileStorage,
		Discard:   nats.DiscardOld,
		MaxAge:    72 * time.Hour,
	}
}

// EnsureStream creates the stream if it is missing.
func EnsureStream(ctx context.Context, js JetStreamAdmin, spec StreamSpec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if _, err := js.StreamInfo(spec.Name); err == nil {
		return nil
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("nats: stream info %q: %w", spec.Name, err)
	}

	_, err := js.AddStream(&nats.StreamConfig{
		Name:        spec.Name,
		Description: spec.Description,
		Subjects:    append([]string(nil), spec.Subjects...),
		Retention:   spec.Retention,
		Storage:     spec.Storage,
		Discard:     spec.Discard,
		MaxAge:      spec.MaxAge,
	})
	if err != nil {
		return fmt.Errorf("nats: add stream %q: %w", spec.Name, err)
	}
	return nil
}

// BootstrapDevStream provisions the default BookDB development stream.
func BootstrapDevStream(ctx context.Context, js JetStreamAdmin, streamName string) error {
	return EnsureStream(ctx, js, DevStreamSpec(streamName))
}
