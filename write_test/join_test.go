package xgxerror

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Test that errors.Join exposes all leaves to errors.Is/As and our helpers.
func TestJoin_IsAsAndHelpers(t *testing.T) {
	a := NotFound("user", 1)            // Failure w/ CodeNotFound
	b := Invalid("email", "bad format") // Failure w/ CodeInvalid
	joined := errors.Join(a, b)         // Go 1.20+: Unwrap() []error; Is/As traverse.

	// stdlib traversal must find both leaves
	if !errors.Is(joined, a) || !errors.Is(joined, b) {
		t.Fatalf("errors.Is should match both joined leaves")
	}

	// errors.As should find our concrete type too
	var fa *failureErr
	if !errors.As(joined, &fa) {
		t.Fatalf("errors.As should locate *failureErr in a joined tree")
	}

	// Our Flatten should return both leaves (order not asserted)
	leaves := Flatten(joined)
	if len(leaves) != 2 {
		t.Fatalf("Flatten(joined) leaves=%d, want 2", len(leaves))
	}

	// Root should return some leaf reachable in the graph
	if root := Root(joined); root == nil || !errors.Is(joined, root) {
		t.Fatalf("Root(joined) should return a reachable leaf")
	}
}

// Test that nils are ignored by errors.Join and all-nil input yields nil.
func TestJoin_NilHandling(t *testing.T) {
	var e1 error
	var e2 error
	if got := errors.Join(e1, e2); got != nil {
		t.Fatalf("errors.Join(nil, nil) = %v; want nil", got)
	}
	a := NotFound("user", 2)
	if got := errors.Join(nil, a, nil); !errors.Is(got, a) {
		t.Fatalf("errors.Join should ignore nils and include non-nil leaves")
	}
}

// Test that %w with multiple operands builds a multi-wrap join that Is/As can traverse.
func TestJoin_MultiWrapViaFmt(t *testing.T) {
	a := Invalid("field", "oops")
	b := NotFound("thing", 9)
	err := fmt.Errorf("batch failed: %w and %w", a, b) // multi-%w joins leaves

	// stdlib traversal per Go 1.20+ must locate both
	if !errors.Is(err, a) || !errors.Is(err, b) {
		t.Fatalf("errors.Is must see both leaves of multi-%%w")
	}

	// Our Walk should visit both leaves (count at least 2 distinct failures).
	countFail := 0
	Walk(err, func(e error) bool {
		var fx *failureErr
		if errors.As(e, &fx) {
			countFail++
		}
		return true
	})
	if countFail < 2 {
		t.Fatalf("Walk should visit multiple failure leaves; got %d", countFail)
	}
}

// Test that additional wrapping layers above a join are still traversed.
func TestJoin_NestedWrapping(t *testing.T) {
	leaf := Invalid("age", "too young")
	joined := errors.Join(leaf, NotFound("user", 5))
	wrapped := Wrap(joined, "pipeline failed", "stage", "validate")

	// Both Is/As must see the original leaf through the extra wrapper.
	if !errors.Is(wrapped, leaf) {
		t.Fatalf("errors.Is should traverse wrapper -> join -> leaf")
	}

	var fx *failureErr
	if !errors.As(wrapped, &fx) {
		t.Fatalf("errors.As should locate failure under wrapper")
	}
	ctx := wrapped.Context()
	if got := ctx["stage"]; got != "validate" {
		t.Fatalf("context lost through wrappers; got stage=%v", got)
	}
}

// Test our helpers against a larger join set.
func TestJoin_FlattenMany(t *testing.T) {
	errs := []error{
		NotFound("user", 7),
		Invalid("email", "bad"),
		Conflict("version mismatch"),
	}
	joined := errors.Join(errs...)

	leaves := Flatten(joined)
	if len(leaves) != len(errs) {
		t.Fatalf("Flatten(join %d) => %d leaves; want %d", len(errs), len(leaves), len(errs))
	}

	// Sanity-check that string forms are included when formatting the join.
	s := fmt.Sprintf("%v", joined)
	for _, e := range errs {
		if !strings.Contains(s, e.Error()) {
			t.Fatalf("joined %%v missing leaf: %q in %q", e.Error(), s)
		}
	}
}

// Ensure HasCode and CodeOf behave sensibly across joins.
func TestJoin_CodePredicates(t *testing.T) {
	a := Unavailable("db")
	b := TooManyRequests("api")
	err := errors.Join(a, b)

	if !HasCode(err, CodeUnavailable) || !HasCode(err, CodeTooManyRequests) {
		t.Fatalf("HasCode should detect codes within a join")
	}
	if c := CodeOf(err); c == "" {
		t.Fatalf("CodeOf(join) should return some code present in the chain")
	}

	// Retryable heuristic should return true if any joined leaf is transient.
	if !IsRetryable(err) {
		t.Fatalf("IsRetryable(join(unavailable,429)) should be true")
	}

	reordered := errors.Join(Invalid("field", "bad"), Timeout(3*time.Second))
	if !HasCode(reordered, CodeTimeout) {
		t.Fatalf("HasCode should find CodeTimeout even when not first in join")
	}
	if !IsRetryable(reordered) {
		t.Fatalf("IsRetryable should consider all codes in a join, regardless of order")
	}
}

// Confirm Flatten/Walk handle single errors (non-join) gracefully.
func TestJoin_SingleErrorPath(t *testing.T) {
	one := NotFound("post", 11)
	if leaves := Flatten(one); len(leaves) != 1 || leaves[0] != one {
		t.Fatalf("Flatten(single) should return the error itself")
	}
	seen := false
	Walk(one, func(e error) bool {
		seen = seen || e == one
		return true
	})
	if !seen {
		t.Fatalf("Walk(single) should visit the node")
	}
}
