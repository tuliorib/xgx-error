package xgxerror

import (
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// NOTE: These synctest-backed tests rely on the Go 1.25 virtual time harness to
// provide deterministic scheduling; synctest ships with Go 1.25 and keeps these
// copy-on-write concurrency checks free of sleeps and flakes.

// TestCOW_ConcurrentFluentMethods_Synctest validates that fluent builders are
// non-mutating (copy-on-write) even when used from many goroutines.
// It runs inside a synctest bubble for deterministic scheduling.
func TestCOW_ConcurrentFluentMethods_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		base := NotFound("user", 42).Ctx("lookup", "start", "tenant", "acme")

		const N = 64
		type result struct {
			gid int
			err Error
		}
		results := make(chan result, N)

		for i := 0; i < N; i++ {
			i := i
			go func() {
				// Each goroutine derives a NEW error with its own context key.
				derived := base.With("gid", i).Ctx("phase", "concurrent")
				results <- result{gid: i, err: derived}
			}()
		}

		// Wait until all goroutines are blocked or finished; in this pattern
		// they should all reach send on results (buffered), so Wait is a no-op
		// but it guarantees determinism within the bubble.
		synctest.Wait()

		// Drain results and validate each derived error has its own context,
		// and that the base error remained unchanged.
		seen := make([]bool, N)
		for i := 0; i < N; i++ {
			r := <-results
			seen[r.gid] = true
			ctx := r.err.Context()
			if ctx["gid"] != r.gid {
				t.Fatalf("derived context gid mismatch: got=%v want=%d", ctx["gid"], r.gid)
			}
			if ctx["phase"] != "concurrent" {
				t.Fatalf("derived context missing phase=concurrent: %#v", ctx)
			}
			// Base must still NOT have gid or phase
			if _, ok := base.Context()["gid"]; ok {
				t.Fatalf("base context mutated (gid present)")
			}
			if _, ok := base.Context()["phase"]; ok {
				t.Fatalf("base context mutated (phase present)")
			}
		}
		for i, ok := range seen {
			if !ok {
				t.Fatalf("missing result for gid=%d", i)
			}
		}
	})
}

// TestSynctest_VirtualTime demonstrates that time is virtualized in the bubble:
// timers that would take real seconds complete "instantly" once all goroutines
// are blocked, per synctest semantics.
func TestSynctest_VirtualTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var fired atomic.Bool
		done := make(chan struct{})

		go func() {
			// This would sleep 10s in real time, but inside synctest the fake
			// clock advances as needed when everything is blocked.
			<-time.After(10 * time.Second)
			fired.Store(true)
			close(done)
		}()

		// Block in the bubble until all goroutines are waiting (the timer).
		synctest.Wait()

		// At this point, virtual time should have advanced and the After() has
		// already fired; done should be closed (or closes immediately now).
		select {
		case <-done:
			// ok
		default:
			// If not already closed, allow the bubble to make progress once more.
			synctest.Wait()
			select {
			case <-done:
				// ok
			default:
				t.Fatalf("timer did not fire under synctest virtual time")
			}
		}

		if !fired.Load() {
			t.Fatalf("timer callback did not run")
		}
	})
}
