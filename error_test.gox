package xgxerror

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestConstructors_Basics(t *testing.T) {
	t.Run("NotFound sets code and context", func(t *testing.T) {
		err := NotFound("user", 42)
		if got := CodeOf(err); got != CodeNotFound {
			t.Fatalf("code = %q, want %q", got, CodeNotFound)
		}
		ctx := err.Context()
		if ctx["entity"] != "user" || ctx["id"] != 42 {
			t.Fatalf("context = %#v, want entity=user id=42", ctx)
		}
	})

	t.Run("Internal nil cause does not capture stack", func(t *testing.T) {
		err := Internal(nil)
		fe, ok := err.(*failureErr)
		if !ok {
			t.Fatalf("expected *failureErr from Internal(nil)")
		}
		if fe.cause != nil {
			t.Fatalf("Internal(nil) should not set a cause; got %v", fe.cause)
		}
		if len(fe.stk) != 0 {
			t.Fatalf("Internal(nil) should not capture a stack; got %d frames", len(fe.stk))
		}
	})

	t.Run("Internal with cause captures stack", func(t *testing.T) {
		cause := errors.New("boom")
		err := Internal(cause)
		fe, ok := err.(*failureErr)
		if !ok {
			t.Fatalf("expected *failureErr from Internal(cause)")
		}
		if fe.cause != cause {
			t.Fatalf("Internal(cause) should retain cause; got %v", fe.cause)
		}
		if len(fe.stk) == 0 {
			t.Fatalf("Internal(cause) should capture a stack")
		}
	})

	t.Run("Defect is defect and unwraps cause", func(t *testing.T) {
		root := errors.New("boom")
		err := Defect(root)
		if !IsDefect(err) {
			t.Fatalf("expected IsDefect true")
		}
		if !errors.Is(err, root) { // stdlib traversal must see cause
			t.Fatalf("errors.Is(err, cause) = false")
		}
		if got := CodeOf(err); got != CodeDefect {
			t.Fatalf("CodeOf(defect) = %q, want %q", got, CodeDefect)
		}
	})

	t.Run("Interrupt unwraps to context.Canceled", func(t *testing.T) {
		err := Interrupt("client canceled")
		if !IsInterrupt(err) {
			t.Fatalf("expected IsInterrupt true")
		}
		if !errors.Is(err, context.Canceled) { // canonical sentinel
			t.Fatalf("errors.Is(err, context.Canceled) = false")
		}
		if got := CodeOf(err); got != CodeInterrupt {
			t.Fatalf("CodeOf(interrupt) = %q, want %q", got, CodeInterrupt)
		}
	})
}

func TestWrapAndFluent_AreNonMutating(t *testing.T) {
	orig := NotFound("user", 7)
	// add context to a NEW error
	aug := orig.Ctx("lookup failed", "tenant", "acme").With("attempt", 2)

	// original must remain unchanged
	if _, ok := orig.Context()["tenant"]; ok {
		t.Fatalf("orig context mutated")
	}
	if orig.Error() == aug.Error() {
		t.Fatalf("expected different messages after Ctx/With")
	}

	// verify copy-on-read map isn't aliasing: mutate returned map shouldn't affect internal
	m := aug.Context()
	m["tenant"] = "evil"
	if aug.Context()["tenant"] != "acme" {
		t.Fatalf("context map must be copy-on-read")
	}
}

func TestFailureCtxVariants(t *testing.T) {
	base := NotFound("user", 1)
	chain := base.Ctx("phase one").Ctx("phase two").Ctx("phase three")

	cf, ok := chain.(*failureErr)
	if !ok {
		t.Fatalf("expected *failureErr from Ctx chain")
	}
	originalMsg := cf.msg

	last := cf.CtxLast("final state", "flag", true)
	lc, ok := last.(*failureErr)
	if !ok {
		t.Fatalf("expected *failureErr from CtxLast")
	}
	if lc.msg != "final state" {
		t.Fatalf("CtxLast should replace message; got %q", lc.msg)
	}
	if got := lc.Context()["flag"]; got != true {
		t.Fatalf("CtxLast context missing flag=true, got=%v", got)
	}
	if cf.msg != originalMsg {
		t.Fatalf("CtxLast mutated original message; got %q want %q", cf.msg, originalMsg)
	}
	if _, ok := cf.Context()["flag"]; ok {
		t.Fatalf("CtxLast should not mutate original context")
	}

	bounded := cf.CtxBound("phase four", 3, "attempt", 4)
	bc, ok := bounded.(*failureErr)
	if !ok {
		t.Fatalf("expected *failureErr from CtxBound")
	}
	if bc.msg != "phase two: phase three: phase four" {
		t.Fatalf("CtxBound should keep most recent segments; got %q", bc.msg)
	}
	if got := bc.Context()["attempt"]; got != 4 {
		t.Fatalf("CtxBound context missing attempt=4, got=%v", got)
	}
	if _, ok := cf.Context()["attempt"]; ok {
		t.Fatalf("CtxBound should not mutate original context")
	}

	reset := cf.CtxBound("reset", 0)
	rc, ok := reset.(*failureErr)
	if !ok {
		t.Fatalf("expected *failureErr from bounded reset")
	}
	if rc.msg != "reset" {
		t.Fatalf("limit <= 0 should behave like CtxLast; got %q", rc.msg)
	}
	if strings.Count(cf.msg, ": ") != 2 {
		t.Fatalf("original message should retain three segments; got %q", cf.msg)
	}
}

func TestStdlibInterop_IsAsJoin(t *testing.T) {
	leafA := NotFound("user", 1)
	leafB := Invalid("email", "bad format")
	joined := errors.Join(leafA, leafB) // Go 1.20+: Unwrap() []error, traversed by Is/As. :contentReference[oaicite:1]{index=1}

	if !errors.Is(joined, leafA) || !errors.Is(joined, leafB) {
		t.Fatalf("errors.Is must see both joined leaves")
	}

	// Our helpers should also traverse both branches
	leaves := Flatten(joined)
	if len(leaves) != 2 {
		t.Fatalf("Flatten(join) leaves=%d, want 2", len(leaves))
	}
}

func TestCtxAndWith_OnNonXgxError(t *testing.T) {
	plain := fmt.Errorf("network down")
	w := Wrap(plain, "fetch profile", "user_id", 99)

	// It becomes an internal failure that wraps the plain error
	if got := CodeOf(w); got != CodeInternal {
		t.Fatalf("CodeOf(wrap non-xgx) = %q, want %q", got, CodeInternal)
	}
	if !errors.Is(w, plain) {
		t.Fatalf("wrapped must report original via errors.Is")
	}
	ctx := w.Context()
	if ctx["user_id"] != 99 {
		t.Fatalf("context lost: %#v", ctx)
	}
}

func TestTimeoutAndRetryable(t *testing.T) {
	err := Timeout(1500 * time.Millisecond)
	if got := CodeOf(err); got != CodeTimeout {
		t.Fatalf("CodeOf(timeout) = %q, want %q", got, CodeTimeout)
	}
	if !IsRetryable(err) {
		t.Fatalf("timeout should be retryable by heuristic")
	}
}

func TestFormatting_DefaultAndVerbose(t *testing.T) {
	cause := errors.New("disk I/O")
	err := Internal(cause).Ctx("persist failed", "op", "write").WithStack()

	// %v should be concise (single line, contains message)
	concise := fmt.Sprintf("%v", err) // formatting contracts per fmt.Formatter. :contentReference[oaicite:2]{index=2}
	if !strings.Contains(concise, "persist failed") {
		t.Fatalf("%%v missing message: %q", concise)
	}

	// %+v should include code, msg=, ctx:, cause:, and stack:
	verbose := fmt.Sprintf("%+v", err)
	for _, want := range []string{
		"code=internal",
		`msg="persist failed`,
		"\nctx:",
		"\ncause:",
		"\nstack:",
	} {
		if !strings.Contains(verbose, want) {
			t.Fatalf("%%+v missing %q in:\n%s", want, verbose)
		}
	}
}

func TestRootAndHasHelpers(t *testing.T) {
	a := NotFound("user", 1)
	b := Invalid("email", "bad")
	err := errors.Join(a, b)

	root := Root(err) // DFS-first leaf
	if root == nil {
		t.Fatal("Root(join) = nil")
	}
	if !(errors.Is(err, root)) {
		t.Fatalf("root should be contained in joined error")
	}

	// Has is a nil-safe shim over errors.Is
	if !Has(err, a.(error)) || !Has(err, b.(error)) {
		t.Fatalf("Has must find both leaves")
	}
}

func TestInterruptDeadline(t *testing.T) {
	err := InterruptDeadline("deadline")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("errors.Is(deadline exceeded) = false")
	}
	if !IsInterrupt(err) {
		t.Fatalf("IsInterrupt must detect deadline")
	}
}

// Sanity: WithStackSkip should skip itself and helper frames enough to produce any stack.
// We only assert that %+v includes a stack section; exact frames are runtime-dependent. :contentReference[oaicite:3]{index=3}
func TestWithStackSkip_ProducesStack(t *testing.T) {
	err := WithStackSkip(New("x"), 0)
	verbose := fmt.Sprintf("%+v", err)
	if !strings.Contains(verbose, "\nstack:") {
		t.Fatalf("expected stack section in verbose formatting:\n%s", verbose)
	}
}
