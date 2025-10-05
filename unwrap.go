// unwrap.go — stdlib-interop helpers for unwrapping errors.
//
// Scope (tiny core):
//   • Generic traversal over single- and multi-wrapped errors.
//   • DFS flattening that cooperates with errors.Join (Unwrap() []error) and
//     classic wrapping (Unwrap() error).
//   • No policy, no logging — just correct, minimal utilities.
//
// Notes:
//   • Go defines two Unwrap forms: Unwrap() error and Unwrap() []error.
//     errors.Is / errors.As already traverse both; these helpers expose a
//     programmatic traversal (e.g., for collecting leaves).
//   • We avoid allocations on fast paths and guard against pathological depth.
package xgxerror

import (
	"errors"
)

// singleUnwrapper matches errors that wrap a single cause.
type singleUnwrapper interface {
	Unwrap() error
}

// multiUnwrapper matches errors that wrap multiple causes (e.g., errors.Join).
type multiUnwrapper interface {
	Unwrap() []error
}

// Flatten walks an error tree and returns a slice of leaf errors in depth-first
// order. For joined errors (Unwrap() []error), all branches are explored.
// nil yields nil.
//
// Guarantees:
//   • The returned slice contains only non-nil errors.
//   • Duplicates may appear if the same leaf is reachable via multiple paths.
//     (Cycle protection prevents infinite loops.)
func Flatten(err error) []error {
	if err == nil {
		return nil
	}

	// Fast path: if it neither unwraps single nor multi, it's already a leaf.
	switch e := err.(type) {
	case multiUnwrapper:
		// continue below
		_ = e
	case singleUnwrapper:
		// continue below
	default:
		return []error{err}
	}

	const maxDepth = 1 << 12 // generous safety cap to avoid runaway graphs
	type frame struct {
		e   error
		idx int // next child index to visit (for multi)
	}

	out := make([]error, 0, 4)
	stack := make([]frame, 0, 8)
	seen := make(map[error]struct{}, 16)

	// push root
	stack = append(stack, frame{e: err})

	for len(stack) > 0 && len(stack) < maxDepth {
		top := &stack[len(stack)-1]

		// If we've seen this error before, pop and continue (cycle guard).
		if _, ok := seen[top.e]; ok {
			stack = stack[:len(stack)-1]
			continue
		}
		seen[top.e] = struct{}{}

		// Try multi-unwrapping first.
		if m, ok := top.e.(multiUnwrapper); ok {
			children := m.Unwrap()
			// Skip nils (per spec, implementations shouldn't include nil).
			for top.idx < len(children) && children[top.idx] == nil {
				top.idx++
			}
			if top.idx < len(children) {
				// DFS: push next child.
				stack = append(stack, frame{e: children[top.idx]})
				top.idx++
				continue
			}
			// No more children → pop.
			stack = stack[:len(stack)-1]
			continue
		}

		// Then try single-unwrapping.
		if s, ok := top.e.(singleUnwrapper); ok {
			if u := s.Unwrap(); u != nil {
				// Descend to single child.
				stack = append(stack, frame{e: u})
				continue
			}
		}

		// Leaf node: record and pop.
		out = append(out, top.e)
		stack = stack[:len(stack)-1]
	}

	return out
}

// Walk traverses an error graph depth-first and calls visit for each distinct
// error encountered (pre-order). If visit returns false, traversal stops early.
// nil is a no-op.
//
// Walk is useful for custom searches without allocating a slice like Flatten.
func Walk(err error, visit func(error) bool) {
	if err == nil || visit == nil {
		return
	}
	const maxDepth = 1 << 12
	type frame struct {
		e   error
		idx int
	}

	stack := make([]frame, 0, 8)
	seen := make(map[error]struct{}, 16)
	stack = append(stack, frame{e: err})

	for len(stack) > 0 && len(stack) < maxDepth {
		top := &stack[len(stack)-1]
		if _, ok := seen[top.e]; ok {
			stack = stack[:len(stack)-1]
			continue
		}
		seen[top.e] = struct{}{}

		if !visit(top.e) {
			return
		}

		// Expand children (multi first, then single) to mirror Flatten.
		if m, ok := top.e.(multiUnwrapper); ok {
			children := m.Unwrap()
			// push in reverse so that we visit children in natural order
			for i := len(children) - 1; i >= 0; i-- {
				if c := children[i]; c != nil {
					stack = append(stack, frame{e: c})
				}
			}
			continue
		}
		if s, ok := top.e.(singleUnwrapper); ok {
			if u := s.Unwrap(); u != nil {
				stack = append(stack, frame{e: u})
				continue
			}
		}
		// Leaf → pop
		stack = stack[:len(stack)-1]
	}
}

// Root returns an arbitrary leaf (deepest along the first-found path). For
// classic singly-wrapped chains, this is the ultimate cause. For joined errors,
// the return value is the first leaf discovered by DFS. If err is nil, Root
// returns nil.
func Root(err error) error {
	leaves := Flatten(err)
	if len(leaves) == 0 {
		return nil
	}
	return leaves[0]
}

// Has reports whether target is found anywhere in err's unwrap graph.
// It is a convenience thin wrapper over errors.Is that also treats nil safely.
func Has(err, target error) bool {
	if err == nil || target == nil {
		return false
	}
	return errors.Is(err, target)
}
