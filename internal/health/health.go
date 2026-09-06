// Package health implements BookDB's health, startup, and readiness endpoints.
//
// Endpoints (see docs/17_API_AND_WEBUI_SPECIFICATION.md):
//
//	GET /health/live     -- liveness (process is up); no dependency probing
//	GET /health/ready    -- readiness; all required deps healthy (or degraded
//	                       within configured tolerance)
//	GET /health/startup  -- startup probe (used by orchestrators while booting)
//
// Readiness reflects required dependency health appropriately:
//
//	Postgres     -> required for readiness (writes)
//	NATS         -> not required for read-path readiness, but reported
//	Valkey       -> degraded-tolerated (performance fallback)
//	OpenSearch   -> not required for exact-ID endpoints; report degraded state
//	S3           -> not required for metadata queries; report degraded state
package health

import (
	"context"
	"sync"
	"time"
)

// Status is the state of a single dependency check.
type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
)

// Checker is the interface a dependency health check must implement.
type Checker interface {
	// Check probes the dependency and returns its status. Implementations must
	// be non-destructive and safe to call concurrently.
	Check(ctx context.Context) Status
}

// CheckFunc adapts a function into a Checker.
type CheckFunc func(context.Context) Status

// Check calls f(ctx).
func (f CheckFunc) Check(ctx context.Context) Status { return f(ctx) }

// Result is a JSON-serializable readiness report.
type Result struct {
	Status       string         `json:"status"`
	Timestamp    time.Time      `json:"timestamp"`
	Version      string         `json:"version"`
	Dependencies map[string]any `json:"dependencies,omitempty"`
}

// Registry tracks named dependency checkers.
type Registry struct {
	mu       sync.RWMutex
	checks   map[string]Checker
	required map[string]bool
}

// NewRegistry creates an empty dependency registry.
func NewRegistry() *Registry {
	return &Registry{
		checks:   make(map[string]Checker),
		required: make(map[string]bool),
	}
}

// Register adds a dependency checker.
func (r *Registry) Register(name string, c Checker, required bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = c
	r.required[name] = required
}

// RunAll executes every registered check concurrently and returns the results.
func (r *Registry) RunAll(ctx context.Context) map[string]any {
	r.mu.RLock()
	chk := make(map[string]Checker, len(r.checks))
	req := make(map[string]bool, len(r.checks))
	for n := range r.checks {
		chk[n] = r.checks[n]
		req[n] = r.required[n]
	}
	r.mu.RUnlock()

	results := make(map[string]any, len(chk))
	var resultsMu sync.Mutex
	var wg sync.WaitGroup
	for n, c := range chk {
		wg.Add(1)
		go func(name string, ch Checker) {
			defer wg.Done()
			st := ch.Check(ctx)
			result := map[string]any{
				"status":   string(st),
				"required": req[name],
			}
			resultsMu.Lock()
			results[name] = result
			resultsMu.Unlock()
		}(n, c)
	}
	wg.Wait()
	return results
}

// Overall computes an aggregate status from the individual dependency results.
// A required dependency being down makes the aggregate down.
func Overall(results map[string]any) Status {
	aggregate := StatusOK
	for name, v := range results {
		_ = name
		m, _ := v.(map[string]any)
		if m == nil {
			continue
		}
		req, _ := m["required"].(bool)
		st, _ := m["status"].(string)
		switch Status(st) {
		case StatusDown:
			if req {
				return StatusDown
			}
			aggregate = StatusDegraded
		case StatusDegraded:
			if req {
				return StatusDegraded
			}
			if aggregate == StatusOK {
				aggregate = StatusDegraded
			}
		}
	}
	return aggregate
}

// Result builds a snapshot from the current dependency checks.
func (r *Registry) Result(ctx context.Context, version string) Result {
	deps := r.RunAll(ctx)
	return Result{
		Status:       string(Overall(deps)),
		Timestamp:    time.Now().UTC(),
		Version:      version,
		Dependencies: deps,
	}
}
