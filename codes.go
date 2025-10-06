// codes.go — minimal, reusable error code definitions for xgx-error core.
//
// Intent:
//   - Provide a small set of widely useful, human-readable codes.
//   - Keep semantics open-ended: no HTTP/status/retry policy in core.
//   - Allow projects to extend with their own codes without a central registry.
//
// Conventions (documented, not enforced here):
//   - Codes are lowercase snake_case ASCII.
//   - Avoid the empty string for custom codes; it is never a built-in.
//   - Higher-level modules (e.g., xgx-error-http, xgx-error-retry) may interpret codes;
//     core does not attach policy or retry semantics.
package xgxerror

// NOTE: Code type is declared in error.go. Invariants are documented there.

// Domain / validation
const (
	CodeBadRequest      Code = "bad_request"
	CodeUnauthorized    Code = "unauthorized"
	CodeForbidden       Code = "forbidden"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeInvalid         Code = "invalid"
	CodeUnprocessable   Code = "unprocessable"
	CodeTooManyRequests Code = "too_many_requests"
)

// Availability / time
const (
	CodeTimeout     Code = "timeout"
	CodeUnavailable Code = "unavailable"
)

// Internal / meta
const (
	CodeInternal  Code = "internal"
	CodeDefect    Code = "defect"
	CodeInterrupt Code = "interrupt"
)

// allBuiltinCodes is the ordered set of codes the core ships with.
// Unexported to avoid exposing mutable slice identity to callers.
// Order is stable to minimize churn in docs/examples.
var allBuiltinCodes = []Code{
	// Domain / validation (8)
	CodeBadRequest,
	CodeUnauthorized,
	CodeForbidden,
	CodeNotFound,
	CodeConflict,
	CodeInvalid,
	CodeUnprocessable,
	CodeTooManyRequests,

	// Availability / time (2)
	CodeTimeout,
	CodeUnavailable,

	// Internal / meta (3)
	CodeInternal,
	CodeDefect,
	CodeInterrupt,
}

// builtinCodeSet provides O(1) membership checks for built-ins.
// Declared via composite literal to avoid runtime init loops.
var builtinCodeSet = map[Code]struct{}{
	CodeBadRequest:      {},
	CodeUnauthorized:    {},
	CodeForbidden:       {},
	CodeNotFound:        {},
	CodeConflict:        {},
	CodeInvalid:         {},
	CodeUnprocessable:   {},
	CodeTooManyRequests: {},
	CodeTimeout:         {},
	CodeUnavailable:     {},
	CodeInternal:        {},
	CodeDefect:          {},
	CodeInterrupt:       {},
}

// BuiltinCodes returns a defensive copy of the built-in codes in a stable order.
func BuiltinCodes() []Code {
	out := make([]Code, len(allBuiltinCodes))
	copy(out, allBuiltinCodes)
	return out
}

// IsBuiltin reports whether c is one of the built-in core codes.
// This is ergonomics-only; projects may define and use custom codes freely.
func (c Code) IsBuiltin() bool {
	_, ok := builtinCodeSet[c]
	return ok
}
