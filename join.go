// join.go — thin, stdlib-first helpers for composing multiple errors.
//
// Philosophy:
//   • Defer to the standard library for multi-error composition.
//   • Keep helpers tiny and predictable; no new wrapper types.
//   • Preserve perfect interop with errors.Is / errors.As / fmt (%w).
//
// Background:
//   • Go 1.20+ introduced multi-error wrapping via errors.Join. The returned
//     value implements Unwrap() []error so Is/As traverse all leaves.
//   • We expose small helpers that encourage using the stdlib shape directly.
package xgxerror

import "errors"

// Join composes zero or more errors into one using the standard library.
//
// Behavior (per errors.Join):
//   • nil inputs are ignored.
//   • Join() with all-nil inputs returns nil.
//   • The returned error implements Unwrap() []error so errors.Is/As traverse
//     all leaves transitively (including nested joins).
//
// Rationale:
//   • By delegating to errors.Join, we keep formatting/semantics consistent
//     with the rest of the Go ecosystem and avoid bespoke container types.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// Append joins 'err' with additional errors. It is equivalent to:
//
//   Join(append([]error{err}, more...)...)
//
// but avoids an allocation in the common nil/singleton cases.
func Append(err error, more ...error) error {
	switch {
	case err == nil && len(more) == 0:
		return nil
	case err == nil && len(more) == 1:
		return more[0]
	default:
		// errors.Join already ignores nils, so we can pass through directly.
		all := make([]error, 0, 1+len(more))
		if err != nil {
			all = append(all, err)
		}
		all = append(all, more...)
		return errors.Join(all...)
	}
}
