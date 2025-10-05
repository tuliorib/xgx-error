// Copyright (c) 2025.
// SPDX-License-Identifier: MIT
//
// See the LICENSE file in the project root for license information.

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
// Notes on semantics (normative):
//   - Message chaining (Ctx): if the current message is empty, it becomes msg;
//     if msg is empty, message is unchanged (but kv fields are still added);
//     otherwise message = old + ": " + msg.
//   - Context fields (Ctx/CtxBound/With): appended in call order as key/value
//     pairs. Non-string "key" causes the entire pair (key and its following
//     value, if any) to be dropped to avoid misalignment. A trailing key with
//     no value is recorded as (key, nil).
//   - Bounded context (CtxBound): enforces a maximum number of total fields;
//     when exceeded, newest fields are kept and the oldest are dropped until
//     total <= maxFields. New fields from kv are added first, then truncation
//     is applied if needed.
//   - Stack capture: WithStack() attempts to skip internal helpers so captured
//     frames begin at or near the user call site. Depending on inlining and
//     tooling, 1–2 boundary frames may still appear.
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
	// error provides the canonical concise message string. Keep it concise;
	// rich export (JSON, structured logs) belongs to adapters outside the core.
	error

	// Ctx attaches a short contextual message and optional key-value fields.
	// Keys should be snake_case for consistency. Returns a NEW Error.
	//
	// Example:
	//   err = err.Ctx("query failed", "table", "users", "elapsed_ms", 12.7)
	Ctx(msg string, kv ...any) Error

	// CtxBound behaves like Ctx but enforces a maximum number of total context
	// fields. When the total would exceed maxFields, it keeps the newest fields
	// and drops the oldest until total <= maxFields. If maxFields <= 0, no
	// bound is applied. Returns a NEW Error.
	//
	// Example:
	//   err = err.CtxBound("retry", 8, "attempt", n, "backoff_ms", d.Milliseconds())
	CtxBound(msg string, maxFields int, kv ...any) Error

	// With adds a single key-value field. Returns a NEW Error.
	//
	// Example:
	//   err = err.With("user_id", 42)
	With(key string, val any) Error

	// Code sets or overrides the classification code. Returns a NEW Error.
	//
	// Example:
	//   err = err.Code(Code("not_found"))
	Code(c Code) Error

	// CodeVal returns the current classification code, or "" if unset.
	CodeVal() Code

	// WithStack returns a new Error that includes a captured stack trace
	// starting at the call site. Implementations SHOULD bound the number of
	// captured frames for performance.
	WithStack() Error

	// WithStackSkip behaves like WithStack but skips an additional number of
	// stack frames above the implementation’s default internal skips. This is
	// useful to hide adapter/helper frames in wrappers.
	WithStackSkip(skip int) Error

	// Context returns a new map containing the structured context fields, or
	// nil if there are none. The map is a copy; mutating it does not affect
	// the Error (copy-on-read).
	Context() map[string]any

	// Unwrap returns the immediate cause (if any) to support errors.Is/As.
	Unwrap() error
}
