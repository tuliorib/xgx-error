// codes.go — minimal, reusable error code definitions for xgx-error core.
//
// Intent:
//   - Provide a small set of widely useful, human-readable codes.
//   - Keep semantics open-ended: no HTTP/status/retry policy in core.
//   - Allow projects to extend with their own codes without a central registry.
//
// Notes:
//   - Codes are strings to preserve stability across logs/JSON and avoid
//     breaking changes from enum edits.
//   - Higher-level modules (e.g., xgx-error-http, xgx-error-retry) may
//     interpret these codes; core does not.
package xgxerror

// Built-in codes commonly used across domains and infrastructure.
//
// Groups (informative, not enforced):
//
//   Domain (often analogous to 4xx): bad_request, unauthorized, forbidden,
//   not_found, conflict, invalid, unprocessable, too_many_requests.
//
//   Infrastructure (often analogous to 5xx): internal, timeout, unavailable.
//
//   Special: defect (programming bug), interrupt (cancellation/deadline).
const (
	// Domain-oriented
	CodeBadRequest      Code = "bad_request"
	CodeUnauthorized    Code = "unauthorized"
	CodeForbidden       Code = "forbidden"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeInvalid         Code = "invalid"
	CodeUnprocessable   Code = "unprocessable"     // e.g., validation failed after syntax accepted
	CodeTooManyRequests Code = "too_many_requests" // throttling, quotas

	// Infrastructure-oriented
	CodeInternal    Code = "internal"
	CodeTimeout     Code = "timeout"
	CodeUnavailable Code = "unavailable"

	// Special classifications
	CodeDefect    Code = "defect"    // programmer error / invariant violation
	CodeInterrupt Code = "interrupt" // context cancellation or deadline
)

// String implements fmt.Stringer for convenience in logs/tests.
func (c Code) String() string { return string(c) }

// IsBuiltin reports whether the code is one of the built-in core codes.
// Empty codes return false.
func (c Code) IsBuiltin() bool {
	if c == "" {
		return false
	}
	for _, builtin := range allBuiltinCodes {
		if c == builtin {
			return true
		}
	}
	return false
}

// BuiltinCodes returns a copy of the built-in codes in stable order.
// Callers may freely modify the returned slice without affecting the package.
func BuiltinCodes() []Code {
	out := make([]Code, len(allBuiltinCodes))
	copy(out, allBuiltinCodes)
	return out
}

// Unexported canonical list to avoid exposing mutable package state.
var allBuiltinCodes = []Code{
	CodeBadRequest,
	CodeUnauthorized,
	CodeForbidden,
	CodeNotFound,
	CodeConflict,
	CodeInvalid,
	CodeUnprocessable,
	CodeTooManyRequests,
	CodeInternal,
	CodeTimeout,
	CodeUnavailable,
	CodeDefect,
	CodeInterrupt,
}
