// wrap_test.go — verification of adapter helpers: From / Wrap / With / Recode / WithStack(*).
package xgxerror

import (
	"errors"
	"strings"
	"testing"
)

// ---- small helpers -----------------------------------------------------------

// func asFailure(t *testing.T, e Error) *failureErr {
// 	t.Helper()
// 	f, ok := e.(*failureErr)
// 	if !ok {
// 		t.Fatalf("expected *failureErr, got %T", e)
// 	}
// 	return f
// }

// For WithStackSkip offset testing: a tiny call chain mirroring stack_test.go.
func wsLevel2(skip int, base Error) Error { // direct caller of WithStackSkip(...)
	return WithStackSkip(base, skip)
}
func wsLevel1(skip int, base Error) Error { // one layer higher
	return wsLevel2(skip, base)
}

// ---- tests: From -------------------------------------------------------------

func TestFrom_NilReturnsNil(t *testing.T) {
	t.Parallel()
	if got := From(nil); got != nil {
		t.Fatalf("From(nil) = %v, want nil", got)
	}
}

func TestFrom_XgxReturnsSameInstance(t *testing.T) {
	t.Parallel()
	base := BadRequest("oops")
	if got := From(base); got != base {
		t.Fatalf("From(xgx) must return the same instance; got=%T", got)
	}
}

func TestFrom_PlainWrapsInternal_NoStack(t *testing.T) {
	t.Parallel()
	plain := errors.New("plain")
	got := From(plain)
	f := asFailure(t, got)
	if f.code != CodeInternal {
		t.Fatalf("From(plain) code=%s, want internal", f.code)
	}
	if !errors.Is(f, plain) {
		t.Fatalf("From(plain) should unwrap to plain")
	}
	if len(f.stk) != 0 {
		t.Fatalf("From(plain) should not capture stack (opt-in); got %d frames", len(f.stk))
	}
}

// ---- tests: Wrap -------------------------------------------------------------

func TestWrap_NilCreatesNew(t *testing.T) {
	t.Parallel()
	got := Wrap(nil, "hello", "k", "v")
	f := asFailure(t, got)
	if f.code != CodeInternal || f.msg != "hello" {
		t.Fatalf("Wrap(nil, msg) unexpected code/msg: %s %q", f.code, f.msg)
	}
	if f.Context()["k"] != "v" {
		t.Fatalf("Wrap(nil) should attach ctx k=v; got %v", f.Context())
	}
}

func TestWrap_XgxDelegatesToCtx(t *testing.T) {
	t.Parallel()
	base := BadRequest("start")
	got := Wrap(base, "ignored-by-msg-rules", "extra", 1)
	f := asFailure(t, got)

	// v1 rule: .Ctx(...) does not concatenate messages; sets msg only if empty.
	if f.msg != "start" {
		t.Fatalf("Wrap(xgx) should not grow message chains; got %q", f.msg)
	}
	if f.code != CodeBadRequest {
		t.Fatalf("code changed unexpectedly: %s", f.code)
	}
	if f.Context()["extra"] != 1 {
		t.Fatalf("expected extra=1 in ctx; got %v", f.Context())
	}
}

func TestWrap_PlainWrapsInternalWithContext(t *testing.T) {
	t.Parallel()
	cause := errors.New("boom")
	got := Wrap(cause, "boundary", "attempt", 3)
	f := asFailure(t, got)
	if f.code != CodeInternal || f.msg != "boundary" {
		t.Fatalf("unexpected code/msg: %s %q", f.code, f.msg)
	}
	if !errors.Is(f, cause) {
		t.Fatalf("Wrap(plain) must unwrap to cause")
	}
	if f.Context()["attempt"] != 3 {
		t.Fatalf("missing ctx attempt=3; got %v", f.Context())
	}
	// no stack unless WithStack*
	if len(f.stk) != 0 {
		t.Fatalf("Wrap(plain) should not capture stack; got %d frames", len(f.stk))
	}
}

// ---- tests: With (field) -----------------------------------------------------

func TestWith_NilCreatesNewWithField(t *testing.T) {
	t.Parallel()
	got := With(nil, "k", "v")
	f := asFailure(t, got)
	if f.code != CodeInternal {
		t.Fatalf("With(nil) code=%s, want internal", f.code)
	}
	if f.Context()["k"] != "v" {
		t.Fatalf("With(nil) should attach k=v; got %v", f.Context())
	}
}

func TestWith_XgxDelegates(t *testing.T) {
	t.Parallel()
	base := BadRequest("x")
	got := With(base, "k", 42)
	f := asFailure(t, got)
	if f.code != CodeBadRequest {
		t.Fatalf("code changed unexpectedly: %s", f.code)
	}
	if f.Context()["k"] != 42 {
		t.Fatalf("expected k=42; got %v", f.Context())
	}
}

func TestWith_PlainWrapsWithField(t *testing.T) {
	t.Parallel()
	plain := errors.New("p")
	got := With(plain, "k", true)
	f := asFailure(t, got)
	if f.code != CodeInternal || f.Context()["k"] != true {
		t.Fatalf("unexpected code/ctx: %s %v", f.code, f.Context())
	}
	if !errors.Is(f, plain) {
		t.Fatalf("With(plain) must unwrap to plain")
	}
}

// ---- tests: Recode -----------------------------------------------------------

func TestRecode_NilCreatesNewWithCode(t *testing.T) {
	t.Parallel()
	got := Recode(nil, CodeTimeout)
	f := asFailure(t, got)
	if f.code != CodeTimeout {
		t.Fatalf("Recode(nil, timeout) code=%s", f.code)
	}
}

func TestRecode_XgxDelegatesToCode(t *testing.T) {
	t.Parallel()
	base := BadRequest("x")
	got := Recode(base, CodeConflict)
	f := asFailure(t, got)
	if f.code != CodeConflict {
		t.Fatalf("Recode(xgx) should change code to conflict; got %s", f.code)
	}
}

func TestRecode_PlainWrapsWithCode(t *testing.T) {
	t.Parallel()
	plain := errors.New("p")
	got := Recode(plain, CodeUnavailable)
	f := asFailure(t, got)
	if f.code != CodeUnavailable || !errors.Is(f, plain) {
		t.Fatalf("Recode(plain) mismatch: code=%s unwrap=%v", f.code, errors.Is(f, plain))
	}
}

// ---- tests: WithStack & WithStackSkip ---------------------------------------

func TestWithStack_NilCreatesWithStack(t *testing.T) {
	t.Parallel()
	got := WithStack(nil)
	f := asFailure(t, got)
	if len(f.stk) == 0 {
		t.Fatalf("WithStack(nil) must capture stack")
	}
}

func TestWithStack_XgxDelegates(t *testing.T) {
	t.Parallel()
	base := BadRequest("x")
	got := WithStack(base)
	f := asFailure(t, got)
	if len(f.stk) == 0 {
		t.Fatalf("WithStack(xgx) must capture stack")
	}
	// original unchanged
	if bf := asFailure(t, base); len(bf.stk) != 0 {
		t.Fatalf("original must remain without stack")
	}
}

func TestWithStack_PlainWrapsWithStack(t *testing.T) {
	t.Parallel()
	plain := errors.New("p")
	got := WithStack(plain)
	f := asFailure(t, got)
	if f.code != CodeInternal || !errors.Is(f, plain) {
		t.Fatalf("WithStack(plain) mismatch: code=%s unwrap=%v", f.code, errors.Is(f, plain))
	}
	if len(f.stk) == 0 {
		t.Fatalf("WithStack(plain) should capture stack")
	}
}

func TestWithStackSkip_AddsCorrectSkipOffset(t *testing.T) {
	t.Parallel()

	// Base xgx error: WithStackSkip should delegate to .WithStackSkip(skip)
	base := BadRequest("x")

	// skip=0 → first frame should be wsLevel2 (direct caller of WithStackSkip).
	e0 := wsLevel1(0, base)
	f0 := asFailure(t, e0)
	if len(f0.stk) == 0 {
		t.Fatalf("WithStackSkip(skip=0) did not capture stack")
	}
	if !strings.HasSuffix(f0.stk[0].Function, "wsLevel2") {
		t.Fatalf("skip=0: expected first frame wsLevel2; got %q", f0.stk[0].Function)
	}

	// skip=1 → also skip wsLevel2; now first frame should be wsLevel1.
	e1 := wsLevel1(1, base)
	f1 := asFailure(t, e1)
	if len(f1.stk) == 0 {
		t.Fatalf("WithStackSkip(skip=1) did not capture stack")
	}
	if !strings.HasSuffix(f1.stk[0].Function, "wsLevel1") {
		t.Fatalf("skip=1: expected first frame wsLevel1; got %q", f1.stk[0].Function)
	}
}
