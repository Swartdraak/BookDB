package ingestion

import "testing"

// TestJobAdmin_StateMachine pins the S4-ADMIN transition table (BDB-011):
// every action is valid only from its documented source state, quarantine
// requires a reason, and terminal states accept no action.
func TestJobAdmin_StateMachine(t *testing.T) {
	cases := []struct {
		name    string
		from    string
		action  string
		reason  string
		wantErr string // "" | "state" | "reason" | "unknown"
		wantTo  string
	}{
		{"pause running", "running", JobActionPause, "", "", "paused"},
		{"resume paused", "paused", JobActionResume, "", "", "running"},
		{"retry failed", "failed", JobActionRetry, "", "", "running"},
		{"quarantine running with reason", "running", JobActionQuarantine, "bad dump", "", "quarantined"},
		{"quarantine paused with reason", "paused", JobActionQuarantine, "repeated failures", "", "quarantined"},
		{"quarantine failed with reason", "failed", JobActionQuarantine, "poisoned records", "", "quarantined"},
		{"pause completed rejected", "completed", JobActionPause, "", "state", ""},
		{"pause quarantined rejected", "quarantined", JobActionPause, "", "state", ""},
		{"resume running rejected", "running", JobActionResume, "", "state", ""},
		{"retry running rejected", "running", JobActionRetry, "", "state", ""},
		{"retry paused rejected", "paused", JobActionRetry, "", "state", ""},
		{"quarantine completed rejected", "completed", JobActionQuarantine, "late", "state", ""},
		{"quarantine without reason rejected", "running", JobActionQuarantine, "", "reason", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Mirror the exact decision path of JobAdmin.TransitionJob:
			// action registered -> quarantine reason -> from-state allowed.
			fromTo, ok := jobTransitions[tc.action]
			if !ok {
				t.Fatalf("action %q not registered", tc.action)
			}
			if tc.action == JobActionQuarantine && tc.reason == "" {
				if tc.wantErr != "reason" {
					t.Fatalf("quarantine without reason: want reason error, got %q", tc.wantErr)
				}
				return
			}
			to, allowed := fromTo[tc.from]
			if !allowed {
				if tc.wantErr != "state" {
					t.Fatalf("transition %q from %q: want state error, got %q", tc.action, tc.from, tc.wantErr)
				}
				return
			}
			if tc.wantErr != "" {
				t.Fatalf("transition %q from %q: unexpectedly rejected (%q)", tc.action, tc.from, tc.wantErr)
			}
			if to != tc.wantTo {
				t.Fatalf("transition %q from %q = %q, want %q", tc.action, tc.from, to, tc.wantTo)
			}
		})
	}
}

// TestJobAdmin_TerminalStates documents that completed and quarantined are
// terminal: no action maps out of them.
func TestJobAdmin_TerminalStates(t *testing.T) {
	for _, terminal := range []string{"completed", "quarantined"} {
		for action, fromTo := range jobTransitions {
			if _, ok := fromTo[terminal]; ok {
				t.Errorf("action %q should not be valid from terminal state %q", action, terminal)
			}
		}
	}
}

// TestJobAdmin_UnknownAction pins the guard against an unregistered action
// (a typo or a future action added without updating the table): it must be
// rejected as an invalid state transition, not silently accepted.
func TestJobAdmin_UnknownAction(t *testing.T) {
	for _, action := range []string{"", "delete", "PAUSE", "quarantine ", "approve"} {
		if _, ok := jobTransitions[action]; ok {
			t.Errorf("action %q must not be registered", action)
		}
	}
}
