// construct.go — semantic constructors & concrete error types for xgx-error core.
//
// Scope (tiny core):
//   - Provide the three core categories as concrete types: Failure, Defect, Interrupt.
//   - Implement the xgxerror.Error interface with NON-MUTATING fluent methods.
//   - Offer pragmatic semantic constructors (domain + infra).
//   - Keep policy out (no logging/HTTP/JSON/retry policy here).
//
// Interop:
//   - errors.Is/As work via Unwrap chains (and stdlib errors.Join for multi-error, elsewhere).
//   - Interrupt unwraps to canonical context errors (context.Canceled / context.DeadlineExceeded).
//
// Notes:
//   - Copy-on-write everywhere: each fluent method returns a fresh value.
//   - Context uses the internal []Field representation from context.go.
//   - Stack capture uses captureStackDefault / captureStack from stack.go.
//
// Formatting & message semantics (v1):
//   - .Ctx(...) and .CtxBound(...) DO NOT concatenate messages; the message stays stable.
//     If msg is empty on the receiver and a non-empty msg is provided, it is set once.
//     Additional details belong in structured context (kv), not in growing ": "-joined strings.
package xgxerror

import (
	"context"
	"fmt"
	"time"
)

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

// failureErr represents an expected, recoverable domain/infrastructure failure.
// Example: not_found, invalid, unavailable.
type failureErr struct {
	msg   string
	code  Code
	ctx   fields
	cause error
	stk   Stack
}

func (e *failureErr) Error() string {
	if e.msg == "" {
		if e.code != "" {
			return string(e.code)
		}
		return "error"
	}
	if e.code != "" {
		return fmt.Sprintf("%s: %s", e.code, e.msg)
	}
	return e.msg
}

func (e *failureErr) Unwrap() error             { return e.cause }
func (e *failureErr) CodeVal() Code             { return e.code }
func (e *failureErr) Context() map[string]any   { return ctxToMap(e.ctx) }

// Ctx attaches optional structured context and, if the current message is empty,
// sets it to the provided msg. It does NOT concatenate messages.
// Use context fields for progressive detail rather than string chaining.
func (e *failureErr) Ctx(msg string, kv ...any) Error {
	n := e.clone()
	if msg != "" && n.msg == "" {
		n.msg = msg
	}
	if len(kv) > 0 {
		n.ctx = ctxCloneAppend(n.ctx, ctxFromKV(kv...)...)
	}
	return n
}

// CtxBound behaves like Ctx but enforces a maximum number of TOTAL context
// fields. When the total would exceed maxFields, it keeps the newest fields and
// drops the oldest until total <= maxFields. If maxFields <= 0, no bound is applied.
//
// Message semantics are identical to Ctx: no concatenation; set once if empty.
func (e *failureErr) CtxBound(msg string, maxFields int, kv ...any) Error {
	n := e.clone()
	if msg != "" && n.msg == "" {
		n.msg = msg
	}
	if len(kv) > 0 {
		n.ctx = ctxCloneAppend(n.ctx, ctxFromKV(kv...)...)
	}
	if maxFields > 0 && len(n.ctx) > maxFields {
		keep := n.ctx[len(n.ctx)-maxFields:]
		// Defensive copy to ensure isolation even if the original had spare capacity.
		copied := make(fields, len(keep))
		copy(copied, keep)
		n.ctx = copied
	}
	return n
}

func (e *failureErr) With(key string, val any) Error {
	n := e.clone()
	n.ctx = ctxCloneAppend(n.ctx, Field{Key: key, Val: val})
	return n
}

func (e *failureErr) Code(c Code) Error {
	n := e.clone()
	n.code = c
	return n
}

func (e *failureErr) WithStack() Error {
	return e.WithStackSkip(0)
}

func (e *failureErr) WithStackSkip(skip int) Error {
	n := e.clone()
	n.stk = captureStackDefault(skip + 1) // +1 to skip this method
	return n
}

func (e *failureErr) clone() *failureErr {
	n := *e
	// defensively copy context slice to preserve immutability guarantees
	if len(e.ctx) > 0 {
		copied := make(fields, len(e.ctx))
		copy(copied, e.ctx)
		n.ctx = copied
	} else {
		n.ctx = emptyFields
	}
	// Stack is an immutable value type (slice of frames); shallow copy is fine.
	return &n
}

// defectErr models an unexpected programming error (bug/invariant violation).
// Always captures a stack at creation for debuggability.
type defectErr struct {
	msg   string
	ctx   fields
	cause error
	stk   Stack
}

func (e *defectErr) Error() string {
	if e.msg != "" {
		return "defect: " + e.msg
	}
	if e.cause != nil {
		return "defect: " + e.cause.Error()
	}
	return "defect"
}

func (e *defectErr) Unwrap() error           { return e.cause }
func (e *defectErr) CodeVal() Code           { return CodeDefect }
func (e *defectErr) Context() map[string]any { return ctxToMap(e.ctx) }

// Ctx: identical message semantics to failureErr — no concatenation.
func (e *defectErr) Ctx(msg string, kv ...any) Error {
	n := e.clone()
	if msg != "" && n.msg == "" {
		n.msg = msg
	}
	if len(kv) > 0 {
		n.ctx = ctxCloneAppend(n.ctx, ctxFromKV(kv...)...)
	}
	return n
}

// CtxBound: identical message semantics; enforces maxFields bound.
func (e *defectErr) CtxBound(msg string, maxFields int, kv ...any) Error {
	n := e.clone()
	if msg != "" && n.msg == "" {
		n.msg = msg
	}
	if len(kv) > 0 {
		n.ctx = ctxCloneAppend(n.ctx, ctxFromKV(kv...)...)
	}
	if maxFields > 0 && len(n.ctx) > maxFields {
		keep := n.ctx[len(n.ctx)-maxFields:]
		copied := make(fields, len(keep))
		copy(copied, keep)
		n.ctx = copied
	}
	return n
}

func (e *defectErr) With(key string, val any) Error {
	n := e.clone()
	n.ctx = ctxCloneAppend(n.ctx, Field{Key: key, Val: val})
	return n
}

// Code ignores attempts to reclassify a defect. Defects are permanently
// CodeDefect to preserve invariants, so this returns a clone without applying
// the supplied code.
func (e *defectErr) Code(c Code) Error { return e.clone() }

func (e *defectErr) WithStack() Error        { return e.clone() } // captured at creation
func (e *defectErr) WithStackSkip(int) Error { return e.clone() } // do not recapture

func (e *defectErr) clone() *defectErr {
	n := *e
	if len(e.ctx) > 0 {
		n.ctx = make(fields, len(e.ctx))
		copy(n.ctx, e.ctx)
	} else {
		n.ctx = emptyFields
	}
	return &n
}

// interruptErr models cooperative cancellation/timeouts. It unwraps to the
// canonical context error so errors.Is(err, context.Canceled) works.
type interruptErr struct {
	msg   string
	ctx   fields
	cause error // either context.Canceled or context.DeadlineExceeded
}

func (e *interruptErr) Error() string {
	if e.msg != "" {
		return "interrupt: " + e.msg
	}
	return "interrupt"
}

func (e *interruptErr) Unwrap() error           { return e.cause }
func (e *interruptErr) CodeVal() Code           { return CodeInterrupt }
func (e *interruptErr) Context() map[string]any { return ctxToMap(e.ctx) }

// Ctx: identical message semantics — no concatenation.
func (e *interruptErr) Ctx(msg string, kv ...any) Error {
	n := e.clone()
	if msg != "" && n.msg == "" {
		n.msg = msg
	}
	if len(kv) > 0 {
		n.ctx = ctxCloneAppend(n.ctx, ctxFromKV(kv...)...)
	}
	return n
}

// CtxBound behaves like Ctx but enforces a maximum number of TOTAL context
// fields. When the total would exceed maxFields, it keeps the newest fields and
// drops the oldest until total <= maxFields. If maxFields <= 0, no bound is applied.
func (e *interruptErr) CtxBound(msg string, maxFields int, kv ...any) Error {
	n := e.clone()
	if msg != "" && n.msg == "" {
		n.msg = msg
	}
	if len(kv) > 0 {
		n.ctx = ctxCloneAppend(n.ctx, ctxFromKV(kv...)...)
	}
	if maxFields > 0 && len(n.ctx) > maxFields {
		keep := n.ctx[len(n.ctx)-maxFields:]
		copied := make(fields, len(keep))
		copy(copied, keep)
		n.ctx = copied
	}
	return n
}

func (e *interruptErr) With(key string, val any) Error {
	n := e.clone()
	n.ctx = ctxCloneAppend(n.ctx, Field{Key: key, Val: val})
	return n
}

func (e *interruptErr) Code(c Code) Error        { return e.clone() } // fixed class
func (e *interruptErr) WithStack() Error         { return e.clone() } // no stacks for interrupts
func (e *interruptErr) WithStackSkip(int) Error  { return e.clone() }

func (e *interruptErr) clone() *interruptErr {
	n := *e
	if len(e.ctx) > 0 {
		n.ctx = make(fields, len(e.ctx))
		copy(n.ctx, e.ctx)
	} else {
		n.ctx = emptyFields
	}
	return &n
}

// -----------------------------------------------------------------------------
// Semantic constructors — Domain (4xx-aligned intent, no HTTP in core)
// -----------------------------------------------------------------------------

// NotFound creates a not_found failure, typically for missing entities.
func NotFound(entity string, id any) Error {
	return &failureErr{
		msg:  fmt.Sprintf("%s not found", entity),
		code: CodeNotFound,
		ctx:  ctxFromKV("entity", entity, "id", id),
	}
}

// Invalid indicates syntactic or semantic invalid input.
func Invalid(field, reason string) Error {
	return &failureErr{
		msg:  "invalid " + field,
		code: CodeInvalid,
		ctx:  ctxFromKV("field", field, "reason", reason),
	}
}

// Unprocessable indicates the request was well-formed but semantically unacceptable.
func Unprocessable(field, reason string) Error {
	return &failureErr{
		msg:  "unprocessable " + field,
		code: CodeUnprocessable,
		ctx:  ctxFromKV("field", field, "reason", reason),
	}
}

func BadRequest(msg string) Error {
	return &failureErr{msg: msg, code: CodeBadRequest, ctx: emptyFields}
}

func Unauthorized(msg string) Error {
	return &failureErr{msg: msg, code: CodeUnauthorized, ctx: emptyFields}
}

func Forbidden(resource string) Error {
	return &failureErr{
		msg:  "forbidden",
		code: CodeForbidden,
		ctx:  ctxFromKV("resource", resource),
	}
}

func Conflict(msg string) Error {
	return &failureErr{msg: msg, code: CodeConflict, ctx: emptyFields}
}

func TooManyRequests(resource string) Error {
	return &failureErr{
		msg:  "too many requests",
		code: CodeTooManyRequests,
		ctx:  ctxFromKV("resource", resource),
	}
}

// -----------------------------------------------------------------------------
// Semantic constructors — Infrastructure (5xx-aligned intent, no HTTP in core)
// -----------------------------------------------------------------------------

// Internal wraps an underlying error as an internal failure and captures a stack.
// If err is nil, returns a generic internal error with a stack capture so the
// boundary is still debuggable.
func Internal(err error) Error {
	fe := &failureErr{
		msg:   "internal error",
		code:  CodeInternal,
		ctx:   emptyFields,
		cause: err,
	}
	return fe.WithStack() // capture once at the boundary
}

// Timeout indicates operation took longer than expected. Records duration.
func Timeout(d time.Duration) Error {
	return &failureErr{
		msg:  "timeout",
		code: CodeTimeout,
		ctx:  ctxFromKV("timeout_ms", float64(d.Milliseconds())),
		// leave cause nil; use InterruptDeadline for canonical context unwrap
	}
}

// Unavailable indicates a transient unavailability (e.g., dependency down).
func Unavailable(service string) Error {
	return &failureErr{
		msg:  "unavailable",
		code: CodeUnavailable,
		ctx:  ctxFromKV("service", service),
	}
}

// -----------------------------------------------------------------------------
// Semantic constructors — Programming defects & cooperative interrupts
// -----------------------------------------------------------------------------

// Defect wraps an unexpected programming error; always captures a stack.
func Defect(err error) Error {
	if err == nil {
		err = fmt.Errorf("nil defect") // avoid nil unwrap surprises
	}
	return &defectErr{
		msg:   "",
		ctx:   emptyFields,
		cause: err,
		stk:   captureStackDefault(0),
	}
}

// Interrupt denotes cooperative cancellation not attributable to defects.
// It unwraps to context.Canceled by default; use InterruptDeadline for timeouts.
func Interrupt(reason string) Error {
	return &interruptErr{
		msg:   reason,
		ctx:   emptyFields,
		cause: context.Canceled,
	}
}

// InterruptDeadline denotes deadline expiration and unwraps to context.DeadlineExceeded.
func InterruptDeadline(reason string) Error {
	return &interruptErr{
		msg:   reason,
		ctx:   emptyFields,
		cause: context.DeadlineExceeded,
	}
}

// -----------------------------------------------------------------------------
// Convenience constructors — Wrapping and ad-hoc creation
// -----------------------------------------------------------------------------

// Ctx wraps an existing error with an additional message and key-values.
// If err already implements xgxerror.Error, it will be augmented immutably.
// Otherwise it becomes an internal failure with 'err' as cause.
//
// Message semantics: same as per-type .Ctx — no concatenation; set once if empty.
func Ctx(err error, msg string, kv ...any) Error {
	if err == nil {
		// Create a generic failure with context only.
		return (&failureErr{msg: msg, code: CodeInternal, ctx: ctxFromKV(kv...)}).clone()
	}
	if xe, ok := err.(Error); ok {
		return xe.Ctx(msg, kv...)
	}
	return (&failureErr{
		msg:   msg,
		code:  CodeInternal,
		ctx:   ctxFromKV(kv...),
		cause: err,
	}).clone()
}

// New creates a new internal failure with a message and optional context.
// Prefer semantic constructors when possible.
func New(msg string, kv ...any) Error {
	return &failureErr{msg: msg, code: CodeInternal, ctx: ctxFromKV(kv...)}
}

// -----------------------------------------------------------------------------
// Interface conformance guards (keep in the file that defines the types)
// -----------------------------------------------------------------------------
var (
	_ Error = (*failureErr)(nil)
	_ Error = (*defectErr)(nil)
	_ Error = (*interruptErr)(nil)
)
