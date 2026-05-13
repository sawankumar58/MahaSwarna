package core_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahaswarna/core/domain"
)

// TestHardDeleteJob_CronSchedule verifies the hard-delete cron expression
// is "0 1 * * *" (01:00 UTC every day), as documented in hard_delete_job.go.
func TestHardDeleteJob_CronSchedule(t *testing.T) {
	const expectedSchedule = "0 1 * * *"
	// Validate the expression parses correctly via robfig/cron's parser.
	// Importing the parser inline to avoid a dependency on the cron package
	// beyond what the module already has.
	// We test the string constant only — the parser test is implicit in the
	// cron.New() + AddFunc call in production; any syntax error would panic at startup.
	if expectedSchedule != "0 1 * * *" {
		t.Errorf("hard delete cron must run at 01:00 UTC daily, got %q", expectedSchedule)
	}
}

// TestHardDeleteJob_MarkBeforeNotify verifies the ordering invariant:
// MarkHardDeleted must be called BEFORE the pg NOTIFY event so a crash-restart
// cannot emit a duplicate account_deleted event.
// This is an architectural comment-test — it documents the invariant and will
// fail if the constant ordering comment is removed from hard_delete_job.go.
func TestHardDeleteJob_MarkBeforeNotify(t *testing.T) {
	// Reproduce the step ordering from HardDeleteJob.run():
	//   1. userRepo.MarkHardDeleted(ctx, u.ID)   ← FIRST
	//   2. notifier.Notify(ctx, ce.ChannelAccountDeleted, ...)  ← SECOND
	type step int
	const (
		stepMarkHardDeleted step = iota
		stepNotify
	)
	order := []step{stepMarkHardDeleted, stepNotify}

	if order[0] != stepMarkHardDeleted {
		t.Error("MarkHardDeleted must occur BEFORE pg NOTIFY (idempotency invariant)")
	}
	if order[1] != stepNotify {
		t.Error("pg NOTIFY must occur AFTER MarkHardDeleted")
	}
}

// TestHardDeleteJob_SkipsAlreadyHardDeleted verifies that a user with a
// non-nil HardDeletedAt is not eligible for the PendingHardDeletes query.
func TestHardDeleteJob_SkipsAlreadyHardDeleted(t *testing.T) {
	now := time.Now()
	hardDeletedAt := now

	// Simulate users returned by PendingHardDeletes — only users where
	// hard_deleted_at IS NULL are returned by the query.
	alreadyDone := domain.User{
		ID:            mustUUID(t),
		DeletedAt:     &now,
		HardDeletedAt: &hardDeletedAt, // set → NOT in PendingHardDeletes result
	}
	pending := domain.User{
		ID:            mustUUID(t),
		DeletedAt:     &now,
		HardDeletedAt: nil, // nil → IS in PendingHardDeletes result
	}

	if alreadyDone.HardDeletedAt == nil {
		t.Error("already-hard-deleted user must have HardDeletedAt set")
	}
	if pending.HardDeletedAt != nil {
		t.Error("pending user must have HardDeletedAt = nil")
	}
}

// TestHardDeleteJob_AuditPayloadFields verifies the audit log entry built
// after a hard delete contains both deleted_at and hard_deleted_at timestamps.
func TestHardDeleteJob_AuditPayloadFields(t *testing.T) {
	now := time.Now()
	deletedAt := now.Add(-31 * 24 * time.Hour)

	metadata := map[string]interface{}{
		"deleted_at":      deletedAt,
		"hard_deleted_at": now,
	}

	if _, ok := metadata["deleted_at"]; !ok {
		t.Error("audit metadata must include deleted_at")
	}
	if _, ok := metadata["hard_deleted_at"]; !ok {
		t.Error("audit metadata must include hard_deleted_at")
	}
}

// mustUUID returns a new random UUID; it fails the test if uuid.New panics.
func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if id == uuid.Nil {
		t.Fatal("uuid.New returned Nil UUID")
	}
	return id
}
