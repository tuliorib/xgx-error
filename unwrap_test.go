// unwrap_test.go — verification of Flatten / Walk / Root / Has semantics.
package xgxerror

import (
	"errors"
	"fmt"
	"testing"
)

// ---------- helpers -----------------------------------------------------------

type leafErr struct{ s string }

func (e leafErr) Error() string { return e.s }

// pointer-typed single wrapper (good for cycles & identity checks)
type wrap1 struct{ cause error }

func (w *wrap1) Error() string { return "single:" + w.cause.Error() }
func (w *wrap1) Unwrap() error { return w.cause }

// pointer-typed multi wrapper so root is a comparable pointer (safe for e==root)
type myJoin struct{ kids []error }

func (j *myJoin) Error() string         { return "join" }
func (j *myJoin) Unwrap() []error       { return j.kids }
func mkJoin(children ...error) *myJoin  { return &myJoin{kids: children} }

// build a single-unwrap chain of length n ending at leaf
func makeChain(n int, leaf error) error {
	e := leaf
	for i := 0; i < n; i++ {
		e = &wrap1{cause: e}
	}
	return e
}

// deep nested join to exceed typical stack depths; uses stdlib errors.Join
func makeDeepJoin(n int) error {
	if n <= 1 {
		return errors.New("L1")
	}
	return errors.Join(makeDeepJoin(n-1), errors.New(fmt.Sprintf("R%d", n)))
}

// ---------- tests: Flatten ----------------------------------------------------

func TestFlatten_NilReturnsNil(t *testing.T) {
	t.Parallel()
	if out := Flatten(nil); out != nil {
		t.Fatalf("Flatten(nil) = %#v, want nil", out)
	}
}

func TestFlatten_SingleErrorReturnsSingleElementSlice(t *testing.T) {
	t.Parallel()
	l := leafErr{"x"}
	out := Flatten(l)
	if len(out) != 1 || !errors.Is(out[0], l) {
		t.Fatalf("Flatten(single) = %v, want [%v]", out, l)
	}
}

func TestFlatten_WrappedChainReturnsDeepestLeaf(t *testing.T) {
	t.Parallel()
	l := leafErr{"deep"}
	chain := makeChain(5, l)
	out := Flatten(chain)
	if len(out) != 1 || !errors.Is(out[0], l) {
		t.Fatalf("Flatten(chain) = %v, want [%v]", out, l)
	}
}

func TestFlatten_JoinReturnsAllLeaves(t *testing.T) {
	t.Parallel()

	l1 := leafErr{"l1"}
	l2 := errors.New("l2") // stdlib leaf
	l3 := leafErr{"l3"}

	// mix wrapped + plain leaves
	joined := errors.Join(&wrap1{cause: l1}, l2, l3)

	out := Flatten(joined)

	var f1, f2, f3 bool
	for _, e := range out {
		if errors.Is(e, l1) {
			f1 = true
		}
		if errors.Is(e, l2) {
			f2 = true
		}
		if errors.Is(e, l3) {
			f3 = true
		}
	}
	if !(f1 && f2 && f3) {
		t.Fatalf("Flatten(join) missing leaves: l1=%v l2=%v l3=%v; out=%v", f1, f2, f3, out)
	}
}

func TestFlatten_DeeplyNestedJoin64PlusDoesNotPanic(t *testing.T) {
	t.Parallel()
	e := makeDeepJoin(70) // deeper than 64
	out := Flatten(e)
	if len(out) == 0 {
		t.Fatalf("Flatten(deep join) returned 0 leaves unexpectedly")
	}
}

func TestFlatten_DetectsCycles_NoInfiniteLoop(t *testing.T) {
	t.Parallel()

	// a -> b -> a cycle with pointer-typed wrappers
	a := &wrap1{}
	b := &wrap1{cause: a}
	a.cause = b

	out := Flatten(a)

	// We only assert termination and boundedness; content is unspecified for pure cycles.
	if len(out) > 1000 {
		t.Fatalf("Flatten(cycle) appears unbounded; len=%d", len(out))
	}
	// Not strictly required to be non-empty, but if it is, it must contain valid errors.
	for _, e := range out {
		if e == nil {
			t.Fatalf("Flatten(cycle) produced nil leaf")
		}
	}
}

// ---------- tests: Walk -------------------------------------------------------

func TestWalk_NilIsNoop(t *testing.T) {
	t.Parallel()
	called := false
	Walk(nil, func(error) bool {
		called = true
		return true
	})
	if called {
		t.Fatalf("Walk(nil, fn) should not call fn")
	}
}

func TestWalk_VisitsAllNodesPreOrder(t *testing.T) {
	t.Parallel()

	// Use pointer-typed root so identity check is safe (no interface panics).
	l1 := leafErr{"l1"}
	l2 := errors.New("l2")
	l3 := leafErr{"l3"}
	root := mkJoin(&wrap1{cause: l1}, l2, l3)

	var seq []string
	Walk(root, func(e error) bool {
		switch {
		case e == root:
			seq = append(seq, "root")
		case errors.Is(e, l1):
			seq = append(seq, "l1")
		case errors.Is(e, l2):
			seq = append(seq, "l2")
		case errors.Is(e, l3):
			seq = append(seq, "l3")
		default:
			seq = append(seq, "node")
		}
		return true
	})

	if len(seq) == 0 || seq[0] != "root" {
		t.Fatalf("pre-order visit expected first 'root'; got %v", seq)
	}
	// Ensure all leaves were visited at least once.
	want := map[string]bool{"l1": false, "l2": false, "l3": false}
	for _, s := range seq {
		if _, ok := want[s]; ok {
			want[s] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Fatalf("Walk did not visit %s; seq=%v", k, seq)
		}
	}
}

func TestWalk_StopsEarlyWhenCallbackReturnsFalse(t *testing.T) {
	t.Parallel()

	l1 := leafErr{"l1"}
	l2 := leafErr{"l2"}
	root := mkJoin(l1, l2)

	count := 0
	Walk(root, func(e error) bool {
		count++
		return false // stop immediately after first visit (the root)
	})
	if count != 1 {
		t.Fatalf("Walk should stop early after first visit; count=%d", count)
	}
}

func TestWalk_HandlesCycles_NoInfiniteLoop(t *testing.T) {
	t.Parallel()

	a := &wrap1{}
	b := &wrap1{cause: a}
	a.cause = b

	count := 0
	Walk(a, func(error) bool {
		count++
		// allow some visits but not unbounded
		return count < 200
	})
	if count == 0 {
		t.Fatalf("Walk(cycle) did not visit any nodes")
	}
	if count >= 200 {
		t.Fatalf("Walk(cycle) appears unbounded; count=%d", count)
	}
}

func TestWalk_VisitsJoinedBranches(t *testing.T) {
	t.Parallel()

	l1 := leafErr{"l1"}
	l2 := leafErr{"l2"}
	root := errors.Join(l1, l2)

	var saw1, saw2 bool
	Walk(root, func(e error) bool {
		if errors.Is(e, l1) {
			saw1 = true
		}
		if errors.Is(e, l2) {
			saw2 = true
		}
		return true
	})
	if !saw1 || !saw2 {
		t.Fatalf("Walk did not visit both joined leaves; l1=%v l2=%v", saw1, saw2)
	}
}

// ---------- tests: Root -------------------------------------------------------

func TestRoot_Nil(t *testing.T) {
	t.Parallel()
	if r := Root(nil); r != nil {
		t.Fatalf("Root(nil) = %v, want nil", r)
	}
}

func TestRoot_SingleReturnsItself(t *testing.T) {
	t.Parallel()
	l := leafErr{"x"}
	if r := Root(l); !errors.Is(r, l) {
		t.Fatalf("Root(single) = %v, want %v", r, l)
	}
}

func TestRoot_ChainReturnsDeepestCause(t *testing.T) {
	t.Parallel()
	l := leafErr{"deep"}
	chain := makeChain(4, l)
	if r := Root(chain); !errors.Is(r, l) {
		t.Fatalf("Root(chain) = %v, want %v", r, l)
	}
}

func TestRoot_JoinReturnsFirstDFSLeaf(t *testing.T) {
	t.Parallel()

	// Use our pointer-typed join so we can guarantee child order and safely
	// reason about DFS-first leaf (left-first).
	left := leafErr{"left"}
	right := leafErr{"right"}
	root := mkJoin(left, right)

	r := Root(root)
	if !errors.Is(r, left) {
		t.Fatalf("Root(join) = %v, want left-first DFS leaf %v", r, left)
	}
}

// ---------- tests: Has --------------------------------------------------------

func TestHas_NilErrOrNilTarget(t *testing.T) {
	t.Parallel()
	if Has(nil, errors.New("x")) {
		t.Fatalf("Has(nil, target) = true, want false")
	}
	if Has(errors.New("x"), nil) {
		t.Fatalf("Has(err, nil) = true, want false")
	}
}

func TestHas_WrapsErrorsIs(t *testing.T) {
	t.Parallel()

	l := leafErr{"z"}
	chain := makeChain(3, l)
	if !Has(chain, l) {
		t.Fatalf("Has(chain, l) = false, want true")
	}
	if !Has(errors.Join(errors.New("a"), l), l) {
		t.Fatalf("Has(join, l) = false, want true")
	}
	if Has(chain, errors.New("nope")) {
		t.Fatalf("Has(chain, nope) = true, want false")
	}
}
