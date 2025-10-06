// wrap.go — tiny, stdlib-friendly wrappers that operate on arbitrary errors.
//
// Purpose
//   - Apply xgxerror’s fluent builders to ANY error value.
//   - Preserve interop with the Go standard library (errors.Is/As/Join).
//   - Stay policy-free: no logging/HTTP/JSON/retry policy here.
//
// Semantics (v1):
//   - From(err):
//       • Pure conversion. If err is nil → returns nil.
//       • If err already implements xgxerror.Error → returned as-is.
//       • Otherwise → wraps as an internal failure (no stack capture).
//   - Wrap(err, msg, kv...):
//       • Adds message/context. If err is nil → creates a NEW failure,
//         because the caller is asserting error-worthy context (not just converting).
//       • If err already implements xgxerror.Error → augmented immutably.
//       • Otherwise → wrapped as an internal failure with provided context.
//   - This asymmetry (From(nil) == nil, Wrap(nil, ...) != nil) is intentional and documented.
package xgxerror

// From converts any error into Error. If err is nil, From returns nil (pure conversion).
//   - nil → nil
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

// Wrap attaches message/context. If err is nil, Wrap creates a new failure error
// because the caller is explicitly asserting error-worthy context (not a pure conversion).
// This asymmetry with From(nil) is intentional and documented.
//   - If err is xgxerror.Error → augmented immutably.
//   - Otherwise → wrapped as internal and attaches context.
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
//   - nil → creates new internal failure with that key/value.
//   - xgxerror.Error → augments immutably.
//   - other → wraps as internal failure and adds key/value.
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
//   - nil → creates new failure with the provided code.
//   - xgxerror.Error → applies code immutably.
//   - other → wraps as internal failure and applies code.
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
