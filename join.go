// join.go — formatting-aware multi-error join for xgx-error core.
//
// Goals:
//   - Preserve stdlib semantics for unwrapping & default string form:
//   - Unwrap() []error for tree traversal (errors.Is/As pre-order DFS).
//   - Error() == newline-joined child Error() strings (like errors.Join).
//   - Improve ergonomics for logs/diagnostics:
//   - Implement fmt.Formatter so "%+v" prints each child with its own "%+v"
//     formatting recursively (codes, ctx, cause, stack), while "%v"/"%s"/"%q"
//     keep the concise stdlib shape.
//
// Package note: prefer xgxerror.Join over errors.Join for better %+v logs;
// Is/As behavior is identical due to Unwrap() []error.
package xgxerror

import (
	"fmt"
	"strings"
)

// multi is a formatting-aware join that mirrors errors.Join for Error()/Unwrap()
// but also implements fmt.Formatter so "%+v" recurses into children.
type multi struct {
	errs []error // non-nil children only
}

// Error concatenates child Error() strings with newlines, identical to errors.Join.
func (m *multi) Error() string {
	switch len(m.errs) {
	case 0:
		return ""
	case 1:
		return m.errs[0].Error()
	default:
		var sb strings.Builder
		for i, e := range m.errs {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(e.Error())
		}
		return sb.String()
	}
}

// Unwrap exposes the children to stdlib traversal (errors.Is/As will walk pre-order).
func (m *multi) Unwrap() []error { return m.errs }

// Format implements fmt.Formatter.
//
//	%v, %s       → render like Error() (concise, stdlib-compatible).
//	%q           → quoted Error() (concise, stdlib-compatible).
//	%+v          → recurse into children and render each with %+v, newline-separated.
func (m *multi) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			for i, e := range m.errs {
				if i > 0 {
					fmt.Fprint(s, "\n")
				}
				// Child may itself implement fmt.Formatter for %+v (xgx errors do).
				fmt.Fprintf(s, "%+v", e)
			}
			return
		}
		fallthrough
	case 's':
		fmt.Fprint(s, m.Error())
	case 'q':
		fmt.Fprintf(s, "%q", m.Error())
	default:
		// Unknown verb: mimic fmt's %-style error for unsupported verbs.
		fmt.Fprintf(s, "%%!%c(%T)", verb, m)
	}
}

// Join returns an error that wraps the given errors, ignoring nils.
// Behavior:
//   - All nil → nil
//   - One non-nil → that error (identity preserved)
//   - 2+ non-nil → *multi (Unwrap() []error), Error() newline-joins like errors.Join
//   - %+v on the returned error prints full recursive details
func Join(errs ...error) error {
	// Filter nils.
	nz := make([]error, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			nz = append(nz, e)
		}
	}
	switch len(nz) {
	case 0:
		return nil
	case 1:
		// Preserve identity for the ergonomic single-element case.
		return nz[0]
	default:
		return &multi{errs: nz}
	}
}

// Append appends more errors to an existing head, optimizing nil cases.
// Behavior mirrors Join semantics while avoiding extra allocations in common paths.
func Append(head error, more ...error) error {
	// Fast paths.
	if head == nil {
		return Join(more...)
	}
	if len(more) == 0 {
		return head
	}
	onlyNil := true
	for _, e := range more {
		if e != nil {
			onlyNil = false
			break
		}
	}
	if onlyNil {
		return head
	}

	// Combine head + more and Join (which filters nils & preserves identity).
	combined := make([]error, 0, 1+len(more))
	combined = append(combined, head)
	combined = append(combined, more...)
	return Join(combined...)
}
