package health

import (
	"context"
	"testing"
)

func TestRegistryResultAndOverall(t *testing.T) {
	r := NewRegistry()
	r.Register("required", CheckFunc(func(context.Context) Status { return StatusOK }), true)
	r.Register("optional", CheckFunc(func(context.Context) Status { return StatusDegraded }), false)

	results := r.RunAll(context.Background())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if got := Overall(results); got != StatusDegraded {
		t.Fatalf("expected degraded overall status, got %s", got)
	}
	report := r.Result(context.Background(), "dev")
	if report.Status != string(StatusDegraded) {
		t.Fatalf("expected degraded snapshot status, got %s", report.Status)
	}
	if report.Version != "dev" {
		t.Fatalf("unexpected version %q", report.Version)
	}
}
