// predicates_test.go — verification of classification and query helpers.
package xgxerror

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIsDefect_MatchesDefectType(t *testing.T) {
	t.Parallel()

	e := Defect(errors.New("boom"))
	if !IsDefect(e) {
		t.Fatalf("IsDefect(defect) = false, want true")
	}
}

func TestIsDefect_MatchesCodeDefectOnNonDefectError(t *testing.T) {
	t.Parallel()

	// Force a failureErr but with CodeDefect to ensure code-based detection works.
	e := BadRequest("x").Code(CodeDefect)
	if !IsDefect(e) {
		t.Fatalf("IsDefect(failure with CodeDefect) = false, want true")
	}
}

func TestIsDefect_Nil(t *testing.T) {
	t.Parallel()

	if IsDefect(nil) {
		t.Fatalf("IsDefect(nil) = true, want false")
	}
}

func TestIsInterrupt_MatchesInterruptType(t *testing.T) {
	t.Parallel()

	e := Interrupt("stop")
	if !IsInterrupt(e) {
		t.Fatalf("IsInterrupt(Interrupt) = false, want true")
	}
}

func TestIsInterrupt_DetectsCanceled(t *testing.T) {
	t.Parallel()

	e := Interrupt("stop")
	if !errors.Is(e, context.Canceled) || !IsInterrupt(e) {
		t.Fatalf("expected IsInterrupt and errors.Is(..., context.Canceled) to be true")
	}
}

func TestIsInterrupt_DetectsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	e := InterruptDeadline("late")
	if !errors.Is(e, context.DeadlineExceeded) || !IsInterrupt(e) {
		t.Fatalf("expected IsInterrupt and errors.Is(..., context.DeadlineExceeded) to be true")
	}
}

func TestIsInterrupt_Nil(t *testing.T) {
	t.Parallel()

	if IsInterrupt(nil) {
		t.Fatalf("IsInterrupt(nil) = true, want false")
	}
}

func TestHasCode_Single(t *testing.T) {
	t.Parallel()

	e := Conflict("dup")
	if !HasCode(e, CodeConflict) {
		t.Fatalf("HasCode(single, conflict) = false, want true")
	}
}

func TestHasCode_Joined(t *testing.T) {
	t.Parallel()

	e1 := Conflict("c1")
	e2 := Invalid("name", "blank")
	j := Join(e1, e2)

	if !HasCode(j, CodeConflict) {
		t.Fatalf("HasCode(join, conflict) = false, want true")
	}
	if !HasCode(j, CodeInvalid) {
		t.Fatalf("HasCode(join, invalid) = false, want true")
	}
}

func TestHasCode_Absent(t *testing.T) {
	t.Parallel()

	e := BadRequest("x")
	if HasCode(e, CodeTimeout) {
		t.Fatalf("HasCode(bad_request, timeout) = true, want false")
	}
}

func TestHasCode_Nil(t *testing.T) {
	t.Parallel()

	if HasCode(nil, CodeInternal) {
		t.Fatalf("HasCode(nil, ...) = true, want false")
	}
}

func TestIsRetryable_PositiveSet(t *testing.T) {
	t.Parallel()

	rt := []Error{
		Timeout(200 * time.Millisecond),
		Unavailable("db"),
		TooManyRequests("api"),
	}
	for _, e := range rt {
		if !IsRetryable(e) {
			t.Fatalf("IsRetryable(%T) = false, want true", e)
		}
	}
}

func TestIsRetryable_NegativeSet(t *testing.T) {
	t.Parallel()

	nt := []Error{
		BadRequest("x"),
		Unauthorized("x"),
		Forbidden("x"),
		NotFound("user", 1),
		Invalid("f", "bad"),
		Unprocessable("f", "bad"),
		Conflict("x"),
		New("internal-ish"), // CodeInternal by default
	}
	for _, e := range nt {
		if IsRetryable(e) {
			t.Fatalf("IsRetryable(%T) = true, want false", e)
		}
	}
}

func TestIsRetryable_ScansAllBranchesInJoin(t *testing.T) {
	t.Parallel()

	// Left is non-retryable; right is retryable → overall should be true.
	left := Conflict("dup")
	right := Timeout(123 * time.Millisecond)
	j := Join(left, right)

	if !IsRetryable(j) {
		t.Fatalf("IsRetryable(join(conflict, timeout)) = false, want true")
	}
}

func TestCodeOf_AsTraversal_FirstPresentCode(t *testing.T) {
	t.Parallel()

	// Wrap a Code-bearing error inside stdlib wrappers to ensure traversal.
	leaf := Invalid("name", "blank")
	w1 := errors.Join(errors.New("noise"), leaf) // multi-unwrap
	w2 := wrapStdlib("outer", w1)                // single-unwrap

	c := CodeOf(w2)
	if c != CodeInvalid {
		t.Fatalf("CodeOf(...) = %q, want %q", c, CodeInvalid)
	}
}

func TestCodeOf_Nil(t *testing.T) {
	t.Parallel()

	if c := CodeOf(nil); c != "" {
		t.Fatalf("CodeOf(nil) = %q, want empty", c)
	}
}

func TestCodeOf_JoinedPicksPresentCode(t *testing.T) {
	t.Parallel()

	e1 := Conflict("dup")
	e2 := errors.New("plain") // no code here
	e3 := Timeout(250 * time.Millisecond)
	j := Join(e2, e1, e3)

	c := CodeOf(j)
	// We don't require a particular branch order; just ensure we got a real code
	// from one of the joined errors.
	if !(c == CodeConflict || c == CodeTimeout) {
		t.Fatalf("CodeOf(join) = %q, want one of {conflict, timeout}", c)
	}
}

// wrapStdlib creates a classic single-unwrap stdlib wrapper.
// Useful to test errors.As/errors.Is traversal through non-xgx wrappers.
func wrapStdlib(msg string, cause error) error {
	return &stdlibWrap{msg: msg, cause: cause}
}

type stdlibWrap struct {
	msg   string
	cause error
}

func (w *stdlibWrap) Error() string { return w.msg + ": " + w.cause.Error() }
func (w *stdlibWrap) Unwrap() error { return w.cause }
