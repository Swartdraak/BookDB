package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestReviewProposal_ApprovalPublishes pins BDB-011 acceptance criterion 4:
// approval creates ONE published revision/event and a visible approved change
// to the canonical catalog.
func TestReviewProposal_ApprovalPublishes(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "pub-approve-pass-1"

	admin := newS4User(t, db, fmt.Sprintf("pub_admin_%d", time.Now().UnixNano()), string(RoleAdministrator), password)
	contrib := newS4User(t, db, fmt.Sprintf("pub_contrib_%d", time.Now().UnixNano()), string(RoleContributor), password)

	// Insert a real person entity so the publication has a target.
	var personID uuid.UUID
	err := db.QueryRowContext(ctx, `INSERT INTO bookdb.people (display_name) VALUES ('Original Person Name') RETURNING person_id`).Scan(&personID)
	if err != nil {
		t.Fatalf("insert person: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.people WHERE person_id = $1`, personID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.change_feed WHERE entity_id = $1`, personID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.outbox WHERE aggregate_id = $1`, personID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.canonical_revisions WHERE entity_id = $1`, personID)
	})

	newName := "Approved Person Name"
	proposal, err := service.SubmitProposal(ctx, contrib.UserID, "person",
		personID, "display_name", []byte(fmt.Sprintf("%q", newName)), "typo fix")
	if err != nil {
		t.Fatalf("submit proposal: %v", err)
	}

	// Record the person's current name before approval.
	var before string
	if err := db.QueryRowContext(ctx,
		`SELECT display_name FROM bookdb.people WHERE person_id = $1`, personID).Scan(&before); err != nil {
		t.Fatalf("read person before: %v", err)
	}
	if before != "Original Person Name" {
		t.Fatalf("person name before = %q, want %q", before, "Original Person Name")
	}

	// Approve.
	if err := service.ReviewProposal(ctx, proposal.ProposalID, admin.UserID, true, "looks good"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// The canonical person name must now be the proposed value.
	var after string
	if err := db.QueryRowContext(ctx,
		`SELECT display_name FROM bookdb.people WHERE person_id = $1`, personID).Scan(&after); err != nil {
		t.Fatalf("read person after: %v", err)
	}
	if after != newName {
		t.Fatalf("person name after approval = %q, want %q", after, newName)
	}

	// A change_feed event must have been appended (exactly one for this publication).
	var feedCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.change_feed WHERE entity_id = $1 AND change_type = 'updated'`, personID).Scan(&feedCount); err != nil {
		t.Fatalf("count change_feed: %v", err)
	}
	if feedCount != 1 {
		t.Fatalf("change_feed events for person = %d, want 1", feedCount)
	}

	// An outbox event for the search projection must have been written.
	var outboxCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.outbox WHERE aggregate_id = $1`, personID).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox events for person = %d, want 1", outboxCount)
	}

	// The canonical revision must have been bumped.
	var revision int64
	if err := db.QueryRowContext(ctx,
		`SELECT revision FROM bookdb.canonical_revisions WHERE entity_type = 'person' AND entity_id = $1`, personID).Scan(&revision); err != nil {
		t.Fatalf("read canonical revision: %v", err)
	}
	if revision < 1 {
		t.Fatalf("canonical revision = %d, want >= 1", revision)
	}

	// An audit event must have been recorded.
	var auditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.audit_events WHERE user_id = $1 AND action = 'proposal.approved'`, admin.UserID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit events for admin = %d, want 1", auditCount)
	}
}

// TestReviewProposal_RejectionLeavesPublicUnchanged pins BDB-011 acceptance
// criterion 4: rejection leaves the public record unchanged.
func TestReviewProposal_RejectionLeavesPublicUnchanged(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "pub-reject-pass-1"

	admin := newS4User(t, db, fmt.Sprintf("rej_admin_%d", time.Now().UnixNano()), string(RoleAdministrator), password)
	contrib := newS4User(t, db, fmt.Sprintf("rej_contrib_%d", time.Now().UnixNano()), string(RoleContributor), password)

	var personID uuid.UUID
	if err := db.QueryRowContext(ctx, `INSERT INTO bookdb.people (display_name) VALUES ('Original Name') RETURNING person_id`).Scan(&personID); err != nil {
		t.Fatalf("insert person: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.people WHERE person_id = $1`, personID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.change_feed WHERE entity_id = $1`, personID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.outbox WHERE aggregate_id = $1`, personID)
	})

	proposal, err := service.SubmitProposal(ctx, contrib.UserID, "person",
		personID, "display_name", []byte(`"Rejected Name"`), "disputed")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := service.ReviewProposal(ctx, proposal.ProposalID, admin.UserID, false, "not convinced"); err != nil {
		t.Fatalf("reject: %v", err)
	}

	// The public record must be UNCHANGED.
	var name string
	if err := db.QueryRowContext(ctx,
		`SELECT display_name FROM bookdb.people WHERE person_id = $1`, personID).Scan(&name); err != nil {
		t.Fatalf("read person: %v", err)
	}
	if name != "Original Name" {
		t.Fatalf("person name after rejection = %q, want %q (unchanged)", name, "Original Name")
	}

	// No change_feed event should have been written.
	var feedCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.change_feed WHERE entity_id = $1`, personID).Scan(&feedCount); err != nil {
		t.Fatalf("count feed: %v", err)
	}
	if feedCount != 0 {
		t.Fatalf("change_feed events after rejection = %d, want 0", feedCount)
	}

	// No outbox event.
	var outboxCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.outbox WHERE aggregate_id = $1`, personID).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("outbox events after rejection = %d, want 0", outboxCount)
	}
}

// TestReviewProposal_ApprovalNonexistentEntity pins BDB-011: approving a
// proposal whose target entity does not exist in the catalog must fail with a
// 404-class error and must NOT mark the proposal approved (the transaction
// rolls back, leaving the proposal pending — no partial publication).
func TestReviewProposal_ApprovalNonexistentEntity(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "pub-nexist-pass-1"

	admin := newS4User(t, db, fmt.Sprintf("ne_admin_%d", time.Now().UnixNano()), string(RoleAdministrator), password)
	contrib := newS4User(t, db, fmt.Sprintf("ne_contrib_%d", time.Now().UnixNano()), string(RoleContributor), password)

	// A person ID that does not exist in bookdb.people.
	ghostID := uuid.New()

	proposal, err := service.SubmitProposal(ctx, contrib.UserID, "person",
		ghostID, "display_name", []byte(`"Ghost Name"`), "typo")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Approving must fail: the target entity does not exist.
	err = service.ReviewProposal(ctx, proposal.ProposalID, admin.UserID, true, "approve ghost")
	if !errors.Is(err, ErrProposalNotFound) {
		t.Fatalf("approve nonexistent entity = %v, want ErrProposalNotFound", err)
	}

	// The proposal must still be PENDING (the review rolled back).
	var status string
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM bookdb.correction_proposals WHERE proposal_id = $1`, proposal.ProposalID).Scan(&status); err != nil {
		t.Fatalf("read proposal status: %v", err)
	}
	if status != "pending" {
		t.Fatalf("proposal status after failed publish = %q, want %q (rolled back)", status, "pending")
	}

	// No change_feed event was written.
	var feedCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.change_feed WHERE entity_id = $1`, ghostID).Scan(&feedCount); err != nil {
		t.Fatalf("count feed: %v", err)
	}
	if feedCount != 0 {
		t.Fatalf("change_feed events for ghost entity = %d, want 0", feedCount)
	}
}

// TestReviewProposal_WorkPublication pins BDB-011: approving a work-title
// correction updates the canonical work title, bumps the work revision, and
// appends a change-feed event.
func TestReviewProposal_WorkPublication(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "pub-work-pass-1"

	admin := newS4User(t, db, fmt.Sprintf("wk_admin_%d", time.Now().UnixNano()), string(RoleAdministrator), password)
	contrib := newS4User(t, db, fmt.Sprintf("wk_contrib_%d", time.Now().UnixNano()), string(RoleContributor), password)

	// A fresh work (inserted + cleaned up by this test) so the publication
	// target is self-contained and never stomps on another package's fixture.
	workID := uuid.New()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO bookdb.works (work_id, canonical_title, normalized_title) VALUES ($1, 'Original Work Title', 'original work title')`,
		workID); err != nil {
		t.Fatalf("insert test work: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.works WHERE work_id = $1`, workID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.change_feed WHERE entity_id = $1`, workID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.outbox WHERE aggregate_id = $1`, workID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.canonical_revisions WHERE entity_id = $1`, workID)
	})

	// The work must exist before the test can read its title.
	var beforeTitle string
	if err := db.QueryRowContext(ctx,
		`SELECT canonical_title FROM bookdb.works WHERE work_id = $1`, workID).Scan(&beforeTitle); err != nil {
		t.Fatalf("read work before: %v", err)
	}

	newTitle := "Corrected Work Title"
	proposal, err := service.SubmitProposal(ctx, contrib.UserID, "work",
		workID, "title", []byte(fmt.Sprintf("%q", newTitle)), "title fix")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := service.ReviewProposal(ctx, proposal.ProposalID, admin.UserID, true, "approve"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	var afterTitle string
	if err := db.QueryRowContext(ctx,
		`SELECT canonical_title FROM bookdb.works WHERE work_id = $1`, workID).Scan(&afterTitle); err != nil {
		t.Fatalf("read work after: %v", err)
	}
	if afterTitle != newTitle {
		t.Fatalf("work title after approval = %q, want %q", afterTitle, newTitle)
	}

	// A change_feed event for the work must exist.
	var feedCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.change_feed WHERE entity_type = 'work' AND entity_id = $1`, workID).Scan(&feedCount); err != nil {
		t.Fatalf("count feed: %v", err)
	}
	if feedCount < 1 {
		t.Fatalf("change_feed events for work = %d, want >= 1", feedCount)
	}

	// The work's canonical revision must have been bumped.
	var revision int64
	if err := db.QueryRowContext(ctx,
		`SELECT revision FROM bookdb.canonical_revisions WHERE entity_type = 'work' AND entity_id = $1`, workID).Scan(&revision); err != nil {
		t.Fatalf("read work revision: %v", err)
	}
	if revision < 1 {
		t.Fatalf("work canonical revision = %d, want >= 1", revision)
	}
}
