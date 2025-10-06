// join_test.go — verification of Join and Append helpers and unwrap traversal.
package xgxerror

import (
	"errors"
	"fmt"
	"testing"
)

type myErr struct{ msg string }

func (e myErr) Error() string { return e.msg }

func TestJoin_AllNilsReturnsNil(t *testing.T) {
	t.Parallel()

	if got := Join(nil, nil, nil); got != nil {
		t.Fatalf("Join(nil,nil,nil) = %v, want nil", got)
	}
}

func TestJoin_IgnoresNilInputs(t *testing.T) {
	t.Parallel()

	e1 := errors.New("one")

	if got := Join(nil, e1); !errors.Is(got, e1) {
		t.Fatalf("Join(nil,e1) should match e1; got %v", got)
	}
	if got := Join(e1, nil); !errors.Is(got, e1) {
		t.Fatalf("Join(e1,nil) should match e1; got %v", got)
	}
}

func TestJoin_SingleNonNilMatchesByIs(t *testing.T) {
	t.Parallel()

	e1 := errors.New("alpha")
	// errors.Join(e1) need not return the same instance, but errors.Is must match.
	if got := Join(e1); !errors.Is(got, e1) {
		t.Fatalf("Join(e1) should match e1 via errors.Is; got %v", got)
	}
}

func TestJoin_MultipleErrorsTraversableByIs(t *testing.T) {
	t.Parallel()

	e1 := fmt.Errorf("wrap-1: %w", myErr{"leaf-1"})
	e2 := errors.New("leaf-2")
	j := Join(e1, e2)

	if !errors.Is(j, e1) {
		t.Fatalf("errors.Is(join, e1)=false; join=%v", j)
	}
	if !errors.Is(j, e2) {
		t.Fatalf("errors.Is(join, e2)=false; join=%v", j)
	}
	// Also confirm the inner myErr is reachable through the wrap.
	if !errors.Is(j, myErr{"leaf-1"}) {
		t.Fatalf("errors.Is(join, myErr{leaf-1})=false; join=%v", j)
	}
}

func TestJoin_MultipleErrorsTraversableByAs(t *testing.T) {
	t.Parallel()

	e1 := fmt.Errorf("wrap-1: %w", myErr{"leaf-1"})
	e2 := errors.New("leaf-2")
	j := Join(e1, e2)

	var target myErr
	if !errors.As(j, &target) {
		t.Fatalf("errors.As(join, *myErr)=false; join=%v", j)
	}
	if target.msg != "leaf-1" {
		t.Fatalf("errors.As yielded unexpected value: %#v", target)
	}
}

func TestAppend_OptimizesNilCasesAndPreservesNilInputs(t *testing.T) {
	t.Parallel()

	// All nils → nil
	if got := Append(nil /* none */); got != nil {
		t.Fatalf("Append(nil) = %v, want nil", got)
	}
	if got := Append(nil, nil); got != nil {
		t.Fatalf("Append(nil,nil) = %v, want nil", got)
	}

	// Single non-nil → match by errors.Is
	e1 := errors.New("e1")
	if got := Append(nil, e1); !errors.Is(got, e1) {
		t.Fatalf("Append(nil,e1) should match e1 via errors.Is; got=%v", got)
	}

	// Mixed nil/non-nil → contains both leaves
	e2 := errors.New("e2")
	a := Append(e1, nil, e2, nil)
	if !errors.Is(a, e1) || !errors.Is(a, e2) {
		t.Fatalf("Append did not preserve inputs: %v", a)
	}

	// Do NOT assert symmetry of errors.Is between arbitrary joined values.
	// errors.Is(a, b) does not imply errors.Is(b, a).
}

func TestJoinedErrors_WorkWithFlatten(t *testing.T) {
	t.Parallel()

	// Build two xgxerror leaves so Flatten (which prefers structured leaves)
	// can discover both reliably.
	leaf1 := Conflict("c1").Ctx("", "k1", 1)           // xgxerror failure
	leaf2 := Invalid("name", "blank").Ctx("", "k2", 2) // xgxerror failure

	// Wrap leaf1 once to ensure Flatten digs through wraps.
	wrapped := fmt.Errorf("wrap: %w", leaf1)
	j := Join(wrapped, leaf2)

	leaves := Flatten(j)

	// We expect both structured leaves to be present.
	// Do NOT assume order.
	var found1, found2 bool
	for _, e := range leaves {
		if errors.Is(e, leaf1) {
			found1 = true
		}
		if errors.Is(e, leaf2) {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatalf("Flatten(join) missing leaves: found1=%v found2=%v; leaves=%v", found1, found2, leaves)
	}
}


func TestJoinedErrors_WorkWithWalk(t *testing.T) {
	t.Parallel()

	leaf1 := myErr{"l1"}
	leaf2 := errors.New("l2")
	wrapped := fmt.Errorf("wrap: %w", leaf1)
	j := Join(wrapped, leaf2)

	var sawLeaf1, sawLeaf2 bool
	var count int
	Walk(j, func(e error) bool {
		count++
		if errors.Is(e, leaf1) {
			sawLeaf1 = true
		}
		if errors.Is(e, leaf2) {
			sawLeaf2 = true
		}
		return true // continue
	})

	if count == 0 {
		t.Fatalf("Walk did not visit any errors")
	}
	if !sawLeaf1 || !sawLeaf2 {
		t.Fatalf("Walk did not reach both leaves: leaf1=%v leaf2=%v", sawLeaf1, sawLeaf2)
	}
}
