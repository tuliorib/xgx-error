// unwrap.go — stdlib-interop helpers for unwrapping errors.
//
// Scope (tiny core):
//   - Generic traversal over single- and multi-wrapped errors.
//   - DFS flattening that cooperates with errors.Join (Unwrap() []error) and
//     classic wrapping (Unwrap() error).
//   - No policy, no logging — just correct, minimal utilities.
//
// Design notes (Go ≥1.20):
//   - errors.Join returns an error with Unwrap() []error; errors.Unwrap only calls
//     Unwrap() error, so correct traversal must handle BOTH forms.
//   - We must NOT use map[error] as a blanket “seen” set: interface values whose
//     dynamic type is not comparable will panic as map keys. We use a dual guard:
//       • seenErr  (map[error]struct{})        — only for comparable dynamic types
//       • seenPtr  (map[uintptr]struct{})      — pointer identity for pointer types
//     Non-comparable, non-pointer dynamics are treated as acyclic (and bounded by depth).
//
// Traversal semantics:
//   - Walk:        pre-order (visit, then expand children). Stops early if fn returns false.
//   - Flatten:     collects LEAVES only (nodes with no children) in DFS order.
//   - Root:        first DFS leaf (deepest along the first path), nil-safe.
//   - Has:         nil-safe wrapper over errors.Is.
//
// Performance:
//   - Reflection is used minimally to decide comparability/pointer identity; this is not
//     on a hot path (error handling), and we add fast-paths for package-local pointer types.
package xgxerror

import (
	"errors"
	"reflect"
)

// single/multi unwrap interfaces (stdlib-compatible)
type singleUnwrapper interface{ Unwrap() error }
type multiUnwrapper interface{ Unwrap() []error }

// ---------- small helpers ----------------------------------------------------

// fastIsPointer returns true if err's dynamic type is a pointer.
// Fast path for xgxerror concrete types; fallback to reflect for others.
func fastIsPointer(err error) bool {
	if err == nil {
		return false
	}
	switch err.(type) {
	case *failureErr, *defectErr, *interruptErr:
		return true
	}
	return reflect.ValueOf(err).Kind() == reflect.Ptr
}

// isComparable reports whether err's dynamic type is comparable (safe as a map key).
func isComparable(err error) bool {
	if err == nil {
		return false
	}
	return reflect.TypeOf(err).Comparable()
}

// ptrID returns a pointer identity for pointer-typed dynamic errors.
func ptrID(err error) (uintptr, bool) {
	if err == nil {
		return 0, false
	}
	rv := reflect.ValueOf(err)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		return rv.Pointer(), true
	}
	return 0, false
}

// markSeen returns true if 'err' was newly marked; false if already seen.
// Uses seenErr for comparable dynamics, seenPtr for pointer-typed non-comparable.
// If err is neither comparable nor pointer, it returns true (treated as acyclic).
func markSeen(err error, seenErr map[error]struct{}, seenPtr map[uintptr]struct{}) bool {
	if err == nil {
		return false
	}
	if isComparable(err) {
		if _, ok := seenErr[err]; ok {
			return false
		}
		seenErr[err] = struct{}{}
		return true
	}
	if fastIsPointer(err) {
		if id, ok := ptrID(err); ok {
			if _, dup := seenPtr[id]; dup {
				return false
			}
			seenPtr[id] = struct{}{}
			return true
		}
	}
	// Non-comparable & non-pointer: allow; bounded by depth cap.
	return true
}

// ---------- API: Flatten / Walk / Root / Has ---------------------------------

// Flatten walks an error graph and returns leaf errors (nodes with no children)
// in depth-first order. It fully explores both single- and multi-unwrap paths.
// If err is nil, it returns nil.
func Flatten(err error) []error {
	if err == nil {
		return nil
	}

	// Fast path: not a wrapper at all → single leaf.
	switch err.(type) {
	case multiUnwrapper, singleUnwrapper:
	default:
		return []error{err}
	}

	const maxDepth = 1 << 12 // generous cap against runaway graphs

	type frame struct {
		e   error
		idx int // next child index to visit (for multi)
	}

	out := make([]error, 0, 4)
	stack := make([]frame, 0, 8)
	seenErr := make(map[error]struct{}, 16)
	seenPtr := make(map[uintptr]struct{}, 16)

	// Seed root
	stack = append(stack, frame{e: err})
	_ = markSeen(err, seenErr, seenPtr)

	for len(stack) > 0 && len(stack) < maxDepth {
		top := &stack[len(stack)-1]

		// Explore multi first; keep node until all children are processed.
		if m, ok := top.e.(multiUnwrapper); ok {
			children := m.Unwrap()
			// Skip nils defensively (should not appear).
			for top.idx < len(children) && children[top.idx] == nil {
				top.idx++
			}
			if top.idx < len(children) {
				child := children[top.idx]
				top.idx++
				if markSeen(child, seenErr, seenPtr) {
					stack = append(stack, frame{e: child})
				}
				continue
			}
			// Done with all children → pop parent.
			stack = stack[:len(stack)-1]
			continue
		}

		// Then single-unwrap; descend IN-PLACE so parents aren't misclassified as leaves.
		if s, ok := top.e.(singleUnwrapper); ok {
			if u := s.Unwrap(); u != nil {
				if markSeen(u, seenErr, seenPtr) {
					top.e = u // descend without pushing; continue exploring
					continue
				}
				// Child already seen: pop parent without recording it as a leaf.
				stack = stack[:len(stack)-1]
				continue
			}
		}

		// Leaf node: record and pop.
		out = append(out, top.e)
		stack = stack[:len(stack)-1]
	}

	return out
}

// Walk traverses an error graph depth-first and calls visit for each DISTINCT
// node in PRE-ORDER (visit BEFORE expanding children). If visit returns false,
// traversal stops early. It is safe on cycles and nil is a no-op.
// Walk visits each distinct node in pre-order (visit before children).
func Walk(err error, visit func(error) bool) {
    if err == nil || visit == nil {
        return
    }
    const maxDepth = 1 << 12
    type frame struct{ e error }

    stack := make([]frame, 0, 8)
    seenErr := make(map[error]struct{}, 16)     // only for comparable dynamics
    seenPtr := make(map[uintptr]struct{}, 16)   // pointer identity for non-comparable pointers

    stack = append(stack, frame{e: err})
    _ = markSeen(err, seenErr, seenPtr)

    for len(stack) > 0 && len(stack) < maxDepth {
        // POP first → guarantees we will not re-visit parent.
        cur := stack[len(stack)-1].e
        stack = stack[:len(stack)-1]

        // Pre-order visit.
        if !visit(cur) {
            return
        }

        // Expand children (multi first; push in reverse for L→R DFS).
        if m, ok := cur.(multiUnwrapper); ok {
            kids := m.Unwrap()
            for i := len(kids) - 1; i >= 0; i-- {
                c := kids[i]
                if c == nil { continue }
                if markSeen(c, seenErr, seenPtr) {
                    stack = append(stack, frame{e: c})
                }
            }
            continue
        }
        if s, ok := cur.(singleUnwrapper); ok {
            if u := s.Unwrap(); u != nil && markSeen(u, seenErr, seenPtr) {
                stack = append(stack, frame{e: u})
            }
            continue
        }
        // Leaf: nothing to push
    }
}


// Root returns the first DFS leaf (deepest along the first path).
// If err is nil, Root returns nil.
func Root(err error) error {
	leaves := Flatten(err)
	if len(leaves) == 0 {
		return nil
	}
	return leaves[0]
}

// Has reports whether target appears anywhere in err's unwrap graph.
// It wraps errors.Is with nil-safety.
func Has(err, target error) bool {
	if err == nil || target == nil {
		return false
	}
	return errors.Is(err, target)
}
