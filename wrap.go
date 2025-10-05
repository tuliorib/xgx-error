// wrap.go — tiny, stdlib-friendly wrappers that operate on arbitrary errors.
//
// Purpose
//   - Apply xgxerror’s fluent builders to ANY error value.
//   - Preserve perfect interop with the Go standard library (errors.Is/As/Join).
//   - Stay policy-free: no logging/HTTP/JSON opinions here.
//
// Background
//   - Go’s error traversal hinges on Unwrap forms: Unwrap() error and, since Go 1.20,
//     Unwrap() []error (used by errors.Join and multi-%w). errors.Is/As traverse both.
//     These helpers keep wrappers minimal and predictable.
//     See Go blog (1.13) and Go 1.20 release notes / pkg docs.
//     References: go1.13 errors blog; pkg.go.dev/errors (Unwrap behavior); Go 1.20 notes.
//
// References
//   - Working with Errors in Go 1.13 — Unwrap/Is/As conventions. :contentReference[oaicite:0]{index=0}
//   - pkg.go.dev/errors — Unwrap only calls Unwrap() error; Join uses Unwrap() []error. :contentReference[oaicite:1]{index=1}
//   - Go 1.20 release notes — multiple wrapping; Is/As updated; multi %w. :contentReference[oaicite:2]{index=2}
package xgxerror

// From converts any error into an xgxerror.Error without adding policy.
//   - nil → nil (contrast Wrap(nil, msg) which creates a fresh failure)
//   - xgxerror.Error → returned as-is
//   - other error → wrapped as internal failure (no stack capture here)
func From(err error) Error {
	if err == nil {
		return nil
	}
	if xe, ok := err.(Error); ok {
		return xe
	}
	// Lightweight wrapper: internal failure, no stack (callers can opt-in).
	return &failureErr{
		msg:   "internal error",
		code:  CodeInternal,
		ctx:   emptyFields,
		cause: err,
	}
}

// Wrap adds a short contextual message and optional key-values to any error.
//   - If err is xgxerror.Error, augments it immutably.
//   - Otherwise wraps err as internal and attaches context.
//
// Prefer semantic constructors (e.g., NotFound/Invalid) when possible.
func Wrap(err error, msg string, kv ...any) Error {
	if err == nil {
		// Create a failure with context only (internal by default).
		return &failureErr{msg: msg, code: CodeInternal, ctx: ctxFromKV(kv...)}
	}
	if xe, ok := err.(Error); ok {
		return xe.Ctx(msg, kv...)
	}
	return &failureErr{
		msg:   msg,
		code:  CodeInternal,
		ctx:   ctxFromKV(kv...),
		cause: err,
	}
}

// With attaches a single key/value to any error immutably.
func With(err error, key string, val any) Error {
	if err == nil {
		return &failureErr{msg: "error", code: CodeInternal, ctx: ctxFromKV(key, val)}
	}
	if xe, ok := err.(Error); ok {
		return xe.With(key, val)
	}
	return &failureErr{
		msg:   "internal error",
		code:  CodeInternal,
		ctx:   ctxFromKV(key, val),
		cause: err,
	}
}

// Recode sets/overrides the classification code on any error immutably.
// For non-xgx errors, it wraps as a failure and applies the code.
func Recode(err error, c Code) Error {
	if err == nil {
		return &failureErr{msg: "error", code: c, ctx: emptyFields}
	}
	if xe, ok := err.(Error); ok {
		return xe.Code(c)
	}
	return &failureErr{
		msg:   "internal error",
		code:  c,
		ctx:   emptyFields,
		cause: err,
	}
}

// WithStack attaches a stack trace to any error immutably.
// For non-xgx errors, it wraps as internal and captures the stack.
func WithStack(err error) Error {
	return WithStackSkip(err, 0)
}

// WithStackSkip attaches a stack while skipping 'skip' frames beyond this call.
// For non-xgx errors, it wraps as internal and captures the stack.
func WithStackSkip(err error, skip int) Error {
	if err == nil {
		return (&failureErr{msg: "error", code: CodeInternal, ctx: emptyFields}).WithStackSkip(skip + 1)
	}
	if xe, ok := err.(Error); ok {
		return xe.WithStackSkip(skip + 1) // +1 to skip this helper
	}
	fe := &failureErr{
		msg:   "internal error",
		code:  CodeInternal,
		ctx:   emptyFields,
		cause: err,
	}
	return fe.WithStackSkip(skip + 1)
}
