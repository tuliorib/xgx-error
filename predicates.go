// predicates.go — minimal, stdlib-aligned predicates for xgx-error core.
//
// Scope:
//   - Zero-policy helpers for common classification questions.
//   - Interop-first: rely on errors.Is / errors.As so traversal works with both
//     Unwrap() error and Unwrap() []error (e.g., errors.Join / multi-%w).
//
// Notes:
//   - Interrupts are detected via errors.Is against context.Canceled /
//     context.DeadlineExceeded (canonical stdlib sentinels).
//   - HasCode / IsRetryable scan the entire unwrap graph (all branches).
//   - CodeOf returns the first discovered Code via errors.As (first match).
//
// Out of scope (by design):
//   - HTTP/status mapping, retry backoff policy, logging.
package xgxerror

import (
	"context"
	"errors"
)

// internal convenience interface for anything that exposes a Code.
type coder interface{ CodeVal() Code }

// IsDefect reports whether err is (or wraps) a programming defect.
//
// It matches either the concrete internal defect type or any value that
// implements CodeVal()==CodeDefect. Traversal follows stdlib rules and
// includes joined graphs.
func IsDefect(err error) bool {
	if err == nil {
		return false
	}
	// Match concrete defect node.
	var d *defectErr
	if errors.As(err, &d) {
		return true
	}
	// Or anything reporting CodeDefect.
	var c coder
	return errors.As(err, &c) && c.CodeVal() == CodeDefect
}

// IsInterrupt reports whether err denotes cooperative cancellation or a deadline.
//
// Returns true if any branch unwraps to context.Canceled or
// context.DeadlineExceeded, or if a node reports CodeInterrupt.
func IsInterrupt(err error) bool {
	if err == nil {
		return false
	}
	// Canonical stdlib sentinels.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Our concrete interrupt node.
	var ie *interruptErr
	if errors.As(err, &ie) {
		return true
	}
	// Or anything reporting CodeInterrupt.
	var c coder
	return errors.As(err, &c) && c.CodeVal() == CodeInterrupt
}

// HasCode reports whether any node in the unwrap graph carries the given code.
// Scans ALL branches (including errors.Join).
func HasCode(err error, code Code) bool {
	if err == nil {
		return false
	}
	found := false
	Walk(err, func(e error) bool {
		if c, ok := e.(coder); ok && c.CodeVal() == code {
			found = true
			return false // early exit
		}
		return true
	})
	return found
}

// IsRetryable is a tiny, policy-free heuristic based on commonly transient codes.
// Returns true if ANY branch reports one of: unavailable, timeout, too_many_requests.
// Backoff/budgets belong in higher layers.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	retryable := false
	Walk(err, func(e error) bool {
		if c, ok := e.(coder); ok {
			switch c.CodeVal() {
			case CodeUnavailable, CodeTimeout, CodeTooManyRequests:
				retryable = true
				return false // early exit
			}
		}
		return true
	})
	return retryable
}

// CodeOf returns the first discovered Code along err's chain (first match)
// or "" if none. Uses errors.As to respect stdlib traversal order.
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var c coder
	if errors.As(err, &c) {
		return c.CodeVal()
	}
	return ""
}
