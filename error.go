// Package xgxerror defines the minimal, composable error model used across
// xgx projects. It focuses on precise classification and context, while
// remaining perfectly interoperable with the Go standard library.
//
// Design tenets:
//   - Interop-first: play nicely with errors.Is/As and errors.Join.
//   - Minimal surface: no logging/HTTP/JSON in core.
//   - Non-mutating ergonomics: fluent builders return a new value.
//   - Selective stacks: callers opt in; defects capture by default (impl detail).
//
// Implementations SHOULD:
//   - Keep fluent methods non-mutating (copy-on-write).
//   - Implement Unwrap() error (and optionally Unwrap() []error on join types)
//     so stdlib traversal (errors.Is/As) observes full causal chains.
//
// See: errors.Is / errors.As / errors.Join contracts in the Go standard library.
package xgxerror

// Code classifies errors into machine-readable categories.
//
// Codes are stringly-typed for stability across serialization boundaries and
// to avoid a central enum with breaking changes. Projects may define their
// own codes in additional packages; the core does not reserve semantics here.
type Code string

// Error is the minimal, fluent, interop-friendly contract for xgx errors.
//
// All fluent methods MUST be non-mutating: they return a new Error value
// (copy-on-write) and MUST NOT alter the receiver state. This guarantees
// thread-safety for shared error values and keeps provenance reproducible for
// logs/tests without external synchronization.
//
// Unwrap semantics:
//   - Implementations SHOULD provide Unwrap() error to expose a causal parent.
//   - Multi-error containers MAY implement Unwrap() []error (in their own file)
//     to integrate with errors.Is/As over joined error trees.
//
// Note: Core intentionally avoids logging/HTTP/JSON methods. Adapters live in
// separate modules (e.g., xgx-error-slog, xgx-error-http, xgx-error-json).
type Error interface {
	// error provides the canonical message string. Keep it concise; rich
	// export (JSON, structured logs) belongs to adapters outside the core.
	error

	// Ctx attaches a short contextual message and optional key-value fields.
	// Keys should be snake_case for consistency. Returns a NEW Error.
	//
	// Example:
	//   err = err.Ctx("query failed", "table", "users", "elapsed_ms", 12.7)
	Ctx(msg string, kv ...any) Error

	// With adds a single key-value field. Returns a NEW Error.
	//
	// Example:
	//   err = err.With("user_id", 42)
	With(key string, val any) Error

	// Code sets or overrides the classification code. Returns a NEW Error.
	//
	// Example:
	//   err = err.Code(Code("not_found"))
	Code(Code) Error

	// WithStack attaches a stack trace to this error. Returns a NEW Error.
	// Implementations SHOULD capture stacks lazily and only when requested.
	WithStack() Error

	// WithStackSkip is like WithStack but allows skipping call frames
	// (e.g., helper wrappers). Returns a NEW Error.
	WithStackSkip(skip int) Error

	// Code returns the classification code. If no code is set, implementations
	// MAY return an empty Code ("") to indicate "unspecified". The getter is
	// named CodeVal to avoid colliding with the fluent Code(Code) Error setter.
	CodeVal() Code

	// Context returns a shallow COPY of the error's context as a map.
	// The returned map MUST be safe to mutate by callers without affecting
	// the stored context (copy-on-read).
	Context() map[string]any

	// Unwrap returns the causal parent error (if any) to enable stdlib
	// traversal via errors.Is/As. Implementations that do not wrap anything
	// SHOULD return nil.
	//
	// Multi-error containers defined elsewhere MAY also implement:
	//   Unwrap() []error
	// which stdlib errors.Is/As will traverse as a set.
	Unwrap() error
}
