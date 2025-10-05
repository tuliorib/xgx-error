// predicates.go — minimal, stdlib-aligned predicates for xgx-error core.
//
// Scope:
//   • Zero-policy helpers that answer common classification questions.
//   • Interop-first: use errors.Is / errors.As so traversal works with both
//     single Unwrap() error and multi Unwrap() []error (e.g., errors.Join).
//
// Notes:
//   • context interrupts: detect via errors.Is against context.Canceled /
//     context.DeadlineExceeded (the canonical values in the stdlib). :contentReference[oaicite:0]{index=0}
//   • Joined errors: errors.Is/As traverse Unwrap() []error as of Go 1.20+. :contentReference[oaicite:1]{index=1}
//
// Out of scope (by design):
//   • HTTP/status mapping, retry backoff policy, logging.
package xgxerror

import (
	"context"
	"errors"
)

// IsDefect reports whether err is (or wraps) a programming defect.
// It matches either the internal defect type or any xgxerror.Error
// that reports CodeDefect.
func IsDefect(err error) bool {
	if err == nil {
		return false
	}
	// Match concrete defect
	var d *defectErr
	if errors.As(err, &d) {
		return true
	}
	// Match any error that exposes CodeVal()==CodeDefect
	var cv interface{ CodeVal() Code }
	if errors.As(err, &cv) {
		return cv.CodeVal() == CodeDefect
	}
	return false
}

// IsInterrupt reports whether err denotes cooperative cancellation or a deadline
// expiry. It returns true if the chain contains xgxerror interrupt OR the
// canonical stdlib context errors.
func IsInterrupt(err error) bool {
	if err == nil {
		return false
	}
	// Canonical stdlib signals. errors.Is handles Unwrap chains (incl. joins).
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) { // stdlib sentinel errors. :contentReference[oaicite:2]{index=2}
		return true
	}
	// Match our internal interrupt type or any CodeInterrupt.
	var ie *interruptErr
	if errors.As(err, &ie) {
		return true
	}
	var cv interface{ CodeVal() Code }
	if errors.As(err, &cv) {
		return cv.CodeVal() == CodeInterrupt
	}
	return false
}

// HasCode reports whether any error in the unwrap graph carries the given code.
func HasCode(err error, code Code) bool {
	if err == nil {
		return false
	}
	var cv interface{ CodeVal() Code }
	return errors.As(err, &cv) && cv.CodeVal() == code // Is/As traverse both single and multi unwraps. :contentReference[oaicite:3]{index=3}
}

// IsRetryable provides a tiny, policy-free heuristic based on codes that
// commonly represent transient conditions. This does NOT implement backoff
// or budgets; higher layers should own retry policy.
//
// Defaults: unavailable, timeout, too_many_requests → retryable.
// Everything else → non-retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	switch CodeOf(err) {
	case CodeUnavailable, CodeTimeout, CodeTooManyRequests:
		return true
	default:
		return false
	}
}

// CodeOf returns the first discovered Code along err's chain, or "" if none.
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var cv interface{ CodeVal() Code }
	if errors.As(err, &cv) {
		return cv.CodeVal()
	}
	return ""
}
