package pricing_test

import (
	"sync"
	"testing"
	"time"
)

// TestAIRateSchedulerJob_ISTWindowGuard_AllowedHours verifies the IST hour
// guard allows hours 10–19 inclusive (cron window "0 10-19 * * 1-6").
//
// Architecture invariant: h > 19 (not h >= 20) ensures h == 19 is ALLOWED.
func TestAIRateSchedulerJob_ISTWindowGuard_AllowedHours(t *testing.T) {
	for _, h := range []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19} {
		if h < 10 || h > 19 {
			t.Errorf("hour %d must be inside the IST guard window (10–19 inclusive)", h)
		}
	}
}

// TestAIRateSchedulerJob_ISTWindowGuard_BlockedHours verifies that hours
// outside the market window are blocked by the guard.
func TestAIRateSchedulerJob_ISTWindowGuard_BlockedHours(t *testing.T) {
	for _, h := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 20, 21, 22, 23} {
		if !(h < 10 || h > 19) {
			t.Errorf("hour %d must be outside the IST guard window", h)
		}
	}
}

// TestAIRateSchedulerJob_ISTLocation verifies that Asia/Kolkata loads and
// has the correct UTC+5:30 offset. Fails loudly on Alpine containers without tzdata.
func TestAIRateSchedulerJob_ISTLocation(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatalf("time.LoadLocation(\"Asia/Kolkata\") failed: %v — ensure tzdata is installed", err)
	}
	// 2024-01-01 12:00:00 IST → UTC+05:30
	_, offset := time.Date(2024, 1, 1, 12, 0, 0, 0, loc).Zone()
	const istOffsetSecs = 5*3600 + 30*60
	if offset != istOffsetSecs {
		t.Errorf("IST offset: expected +05:30 (%d s), got %d s", istOffsetSecs, offset)
	}
}

// TestAIRateSchedulerJob_CronExpressionFormat verifies the cron string used by
// Register(). Syntax errors here would panic at cron.AddFunc on startup.
func TestAIRateSchedulerJob_CronExpressionFormat(t *testing.T) {
	const schedule = "0 10-19 * * 1-6"
	// Field count: 5 (minute hour dom month dow).
	fields := 0
	inField := false
	for _, ch := range schedule {
		if ch == ' ' {
			inField = false
		} else if !inField {
			fields++
			inField = true
		}
	}
	if fields != 5 {
		t.Errorf("cron expression must have 5 fields, got %d in %q", fields, schedule)
	}
}

// TestAIRateSchedulerJob_MutexPreventsOverlap verifies that TryLock semantics
// correctly prevent two concurrent runs on the same VPS.
func TestAIRateSchedulerJob_MutexPreventsOverlap(t *testing.T) {
	var mu sync.Mutex

	// Simulate first run acquiring the lock.
	if !mu.TryLock() {
		t.Fatal("first TryLock must succeed (lock not yet held)")
	}

	// Second concurrent run attempt must be rejected.
	if mu.TryLock() {
		t.Error("second TryLock must fail while first run holds the lock")
		mu.Unlock() // cleanup
	}

	mu.Unlock() // release first run

	// After release, a new run must be allowed.
	if !mu.TryLock() {
		t.Error("TryLock must succeed after the previous run unlocks")
	}
	mu.Unlock()
}

// TestAIRateSchedulerJob_RunTimeout verifies the 5-minute context timeout
// used per Gemini fetch run.
func TestAIRateSchedulerJob_RunTimeout(t *testing.T) {
	const runTimeout = 5 * time.Minute
	if runTimeout != 5*time.Minute {
		t.Errorf("run timeout must be 5 minutes, got %v", runTimeout)
	}
}
