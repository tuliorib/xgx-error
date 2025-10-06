// construct_test.go — verification of semantic constructors, fluent API, and copy-on-write.
package xgxerror

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// helper to extract concrete types in tests
func asFailure(t *testing.T, e Error) *failureErr {
	t.Helper()
	f, ok := e.(*failureErr)
	if !ok {
		t.Fatalf("expected *failureErr, got %T", e)
	}
	return f
}
func asDefect(t *testing.T, e Error) *defectErr {
	t.Helper()
	d, ok := e.(*defectErr)
	if !ok {
		t.Fatalf("expected *defectErr, got %T", e)
	}
	return d
}
func asInterrupt(t *testing.T, e Error) *interruptErr {
	t.Helper()
	it, ok := e.(*interruptErr)
	if !ok {
		t.Fatalf("expected *interruptErr, got %T", e)
	}
	return it
}

func TestConstructors_Semantics_CodeAndMessage(t *testing.T) {
	t.Parallel()

	t.Run("NotFound", func(t *testing.T) {
		e := NotFound("user", 42)
		f := asFailure(t, e)
		if f.code != CodeNotFound {
			t.Fatalf("code: want=%s got=%s", CodeNotFound, f.code)
		}
		if f.msg != "user not found" {
			t.Fatalf("msg: want=%q got=%q", "user not found", f.msg)
		}
		if f.ctx == nil || len(f.ctx) == 0 {
			t.Fatalf("ctx should be populated")
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		e := Invalid("email", "format")
		f := asFailure(t, e)
		if f.code != CodeInvalid || f.msg != "invalid email" {
			t.Fatalf("unexpected code/msg: %s %q", f.code, f.msg)
		}
	})

	t.Run("Unprocessable", func(t *testing.T) {
		e := Unprocessable("payload", "conflict")
		f := asFailure(t, e)
		if f.code != CodeUnprocessable || f.msg != "unprocessable payload" {
			t.Fatalf("unexpected code/msg: %s %q", f.code, f.msg)
		}
	})

	t.Run("BadRequest/Unauthorized/Forbidden/Conflict/TooManyRequests", func(t *testing.T) {
		if asFailure(t, BadRequest("bad")).code != CodeBadRequest {
			t.Fatal("BadRequest code mismatch")
		}
		if asFailure(t, Unauthorized("nope")).code != CodeUnauthorized {
			t.Fatal("Unauthorized code mismatch")
		}
		if asFailure(t, Forbidden("res")).code != CodeForbidden {
			t.Fatal("Forbidden code mismatch")
		}
		if asFailure(t, Conflict("dup")).code != CodeConflict {
			t.Fatal("Conflict code mismatch")
		}
		if asFailure(t, TooManyRequests("res")).code != CodeTooManyRequests {
			t.Fatal("TooManyRequests code mismatch")
		}
	})

	t.Run("Timeout/Unavailable", func(t *testing.T) {
		if asFailure(t, Timeout(123*time.Millisecond)).code != CodeTimeout {
			t.Fatal("Timeout code mismatch")
		}
		if asFailure(t, Unavailable("db")).code != CodeUnavailable {
			t.Fatal("Unavailable code mismatch")
		}
	})
}

func TestConstructors_ContextFieldsPopulated(t *testing.T) {
	t.Parallel()

	e := NotFound("file", "/tmp/a.txt")
	m := e.Context()
	if got := m["entity"]; got != "file" {
		t.Fatalf("entity ctx: want=file got=%v", got)
	}
	if got := m["id"]; got != "/tmp/a.txt" {
		t.Fatalf("id ctx: want=/tmp/a.txt got=%v", got)
	}
}

func TestInternal_StackAndCause(t *testing.T) {
	t.Parallel()

	t.Run("nil cause captures stack", func(t *testing.T) {
		e := Internal(nil)
		f := asFailure(t, e)
		if f.code != CodeInternal {
			t.Fatalf("code: want=%s got=%s", CodeInternal, f.code)
		}
		if len(f.stk) == 0 {
			t.Fatalf("expected stack to be captured for Internal(nil)")
		}
		if f.cause != nil {
			t.Fatalf("expected nil cause when passed nil")
		}
	})

	t.Run("non-nil cause wrapped and stack captured", func(t *testing.T) {
		cause := errors.New("boom")
		e := Internal(cause)
		f := asFailure(t, e)
		if !errors.Is(f, cause) {
			t.Fatalf("expected errors.Is to match cause")
		}
		if len(f.stk) == 0 {
			t.Fatalf("expected stack to be captured for Internal(cause)")
		}
	})
}

func TestDefect_Behavior(t *testing.T) {
	t.Parallel()

	t.Run("Defect(nil) creates 'nil defect' cause", func(t *testing.T) {
		e := Defect(nil)
		d := asDefect(t, e)
		if d.cause == nil {
			t.Fatalf("Defect(nil) should synthesize non-nil cause")
		}
		if msg := d.Error(); msg != "defect: nil defect" {
			t.Fatalf("defect error text mismatch: got %q", msg)
		}
		if len(d.stk) == 0 {
			t.Fatalf("defect must capture stack at creation")
		}
	})

	t.Run("Defect(err) captures stack and unwraps", func(t *testing.T) {
		cause := errors.New("bad invariant")
		e := Defect(cause)
		d := asDefect(t, e)
		if !errors.Is(d, cause) {
			t.Fatalf("expected errors.Is to match cause")
		}
		if len(d.stk) == 0 {
			t.Fatalf("defect must capture stack at creation")
		}
	})
}

func TestInterrupt_UnwrapsCanonical(t *testing.T) {
	t.Parallel()

	if !errors.Is(Interrupt("stop"), context.Canceled) {
		t.Fatalf("Interrupt should unwrap to context.Canceled")
	}
	if !errors.Is(InterruptDeadline("late"), context.DeadlineExceeded) {
		t.Fatalf("InterruptDeadline should unwrap to context.DeadlineExceeded")
	}
}

func TestCtx_MessageOnce_NoConcatenation(t *testing.T) {
	t.Parallel()

	// Start with no message; first Ctx sets it.
	e := BadRequest("").Ctx("first")
	f1 := asFailure(t, e)
	if f1.msg != "first" {
		t.Fatalf("want msg=first got %q", f1.msg)
	}
	// Subsequent Ctx with another message should NOT concatenate.
	e2 := f1.Ctx("second", "k", "v")
	f2 := asFailure(t, e2)
	if f2.msg != "first" {
		t.Fatalf("message should remain stable; got %q", f2.msg)
	}
	// Context appended.
	if v := f2.Context()["k"]; v != "v" {
		t.Fatalf("expected ctx k=v, got %v", f2.Context())
	}
}

func TestCtxBound_TruncatesNewestN_AndKeepsAllWhenZero(t *testing.T) {
	t.Parallel()

	e := BadRequest("oops").
		Ctx("", "a", 1).
		Ctx("", "b", 2).
		Ctx("", "c", 3) // order: a,b,c (newest last)

	// Truncate to newest 2 → keep b,c
	e2 := e.CtxBound("", 2, "d", 4) // add d then truncate to 2 newest: should keep c,d
	f2 := asFailure(t, e2)
	ctx2 := f2.ctx
	if len(ctx2) != 2 {
		t.Fatalf("expected 2 fields after truncation, got %d", len(ctx2))
	}
	if ctx2[0].Key != "c" || ctx2[1].Key != "d" {
		t.Fatalf("expect newest two [c,d], got [%s,%s]", ctx2[0].Key, ctx2[1].Key)
	}

	// Bound == 0 → keep all (no truncation)
	e3 := e.CtxBound("", 0, "x", 9)
	f3 := asFailure(t, e3)
	if len(f3.ctx) != 4 {
		t.Fatalf("expected keep-all with bound=0, got %d", len(f3.ctx))
	}
}

func TestWith_AddsSingleField_Immutably(t *testing.T) {
	t.Parallel()

	e0 := BadRequest("oops")
	f0 := asFailure(t, e0)
	e1 := e0.With("k", "v")
	f1 := asFailure(t, e1)

	// original untouched
	if len(f0.ctx) != 0 {
		t.Fatalf("original should remain with empty ctx; got %v", f0.ctx)
	}
	// new has field
	if len(f1.ctx) != 1 || f1.ctx[0].Key != "k" || f1.ctx[0].Val != "v" {
		t.Fatalf("expected With to add k=v; got %v", f1.ctx)
	}
}

func TestCode_ChangesOnlyForFailures_IgnoredForOthers(t *testing.T) {
	t.Parallel()

	// Failure: code changes
	f := asFailure(t, BadRequest("x").Code(CodeConflict))
	if f.code != CodeConflict {
		t.Fatalf("failure Code() should change code; got %s", f.code)
	}

	// Defect: code ignored
	d := asDefect(t, Defect(fmt.Errorf("x")).Code(CodeConflict))
	if d.CodeVal() != CodeDefect {
		t.Fatalf("defect Code() must remain CodeDefect; got %s", d.CodeVal())
	}

	// Interrupt: code ignored (fixed)
	it := asInterrupt(t, Interrupt("x").Code(CodeConflict))
	if it.CodeVal() != CodeInterrupt {
		t.Fatalf("interrupt Code() must remain CodeInterrupt; got %s", it.CodeVal())
	}
}

func TestWithStack_BehaviorPerType(t *testing.T) {
	t.Parallel()

	// Failure: captures stack
	f0 := asFailure(t, BadRequest("x"))
	f1 := asFailure(t, f0.WithStack())
	if len(f1.stk) == 0 {
		t.Fatalf("failure WithStack must capture stack")
	}
	// original unchanged
	if len(f0.stk) != 0 {
		t.Fatalf("original failure must remain without stack")
	}

	// Defect: already captured; WithStack returns clone without recapture
	d0 := asDefect(t, Defect(fmt.Errorf("y")))
	d1 := asDefect(t, d0.WithStack())
	if &d0 == &d1 {
		t.Fatalf("WithStack should return a clone (new pointer)")
	}
	// we cannot easily compare stacks by pointer, but length should remain > 0
	if len(d1.stk) == 0 || len(d0.stk) == 0 {
		t.Fatalf("defect stacks must exist")
	}

	// Interrupt: no stacks; WithStack returns clone
	i0 := asInterrupt(t, Interrupt("z"))
	i1 := asInterrupt(t, i0.WithStack())
	if &i0 == &i1 {
		t.Fatalf("WithStack should return a clone for interrupts")
	}
}

func TestClone_IndependenceAndEmptyFieldsCanonical(t *testing.T) {
	t.Parallel()

	// Start with no ctx; clone should give emptyFields.
	f0 := asFailure(t, BadRequest("x"))
	f1 := f0.clone()
	if len(f1.ctx) != 0 {
		t.Fatalf("clone should have emptyFields; got %v", f1.ctx)
	}

	// With context: clones must not alias backing arrays.
	f2 := asFailure(t, f0.With("a", 1).With("b", 2))
	cl := f2.clone()
	if &cl.ctx[0] == &f2.ctx[0] {
		t.Fatalf("clone must deep copy context slice (no aliasing)")
	}

	// Mutate clone; original must not change.
	cl.ctx[0].Val = 999
	if f2.ctx[0].Val.(int) != 1 {
		t.Fatalf("copy-on-write violated: original mutated via clone")
	}
}

func TestErrorString_FormatsWithAndWithoutCode(t *testing.T) {
	t.Parallel()

	// With code and msg
	e1 := Conflict("duplicate email")
	if got, want := e1.Error(), "conflict: duplicate email"; got != want {
		t.Fatalf("Error(): want %q got %q", want, got)
	}

	// With msg only (no code set explicitly — but constructor sets code)
	// We simulate by making a failure with empty code.
	f := &failureErr{msg: "just text"}
	if got, want := f.Error(), "just text"; got != want {
		t.Fatalf("Error(): want %q got %q", want, got)
	}

	// With code only (no msg)
	f2 := &failureErr{code: CodeInternal}
	if got, want := f2.Error(), "internal"; got != want {
		t.Fatalf("Error(): want %q got %q", want, got)
	}

	// Neither code nor msg
	f3 := &failureErr{}
	if got, want := f3.Error(), "error"; got != want {
		t.Fatalf("Error(): want %q got %q", want, got)
	}
}

func TestCopyOnWrite_OriginalUnchangedAfterFluentCalls(t *testing.T) {
	t.Parallel()

	e0 := BadRequest("start")
	f0 := asFailure(t, e0)

	e1 := e0.Ctx("ignored second message", "k1", 1)
	e2 := e1.With("k2", 2)
	e3 := e2.Code(CodeConflict)

	// Original must remain the same
	if f0.msg != "start" || f0.code != CodeBadRequest || len(f0.ctx) != 0 {
		t.Fatalf("original mutated: %#v", f0)
	}

	// Final fluent result should reflect cumulative changes (with stable msg)
	f3 := asFailure(t, e3)
	if f3.msg != "start" || f3.code != CodeConflict {
		t.Fatalf("fluent result mismatch: msg=%q code=%s", f3.msg, f3.code)
	}
	if len(f3.ctx) != 2 {
		t.Fatalf("expected 2 ctx fields; got %d", len(f3.ctx))
	}
}
