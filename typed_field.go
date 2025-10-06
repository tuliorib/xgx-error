// typed_field.go — optional, type-safe field helpers for xgx-error core.
//
// Copyright (c) 2025.
// SPDX-License-Identifier: MIT
//
// Overview
//   TypedField provides an *optional* ergonomic layer for attaching and reading
//   strongly-typed context fields on xgx errors. It does not replace the
//   plain string/any API (`With`, `Ctx`, `CtxBound`) — it complements it.
//
// Goals
//   • Zero policy: purely a convenience for authors who prefer typed access.
//   • No lock-in: you can mix `.With("k", any)` with `Field[T]("k").Set/Get`.
//   • Interop-first: works over the public Error interface.
//
// Implementation notes
//   • Get/MustGet read via Error.Context(), which returns a copy (copy-on-read).
//     This means one map allocation per call. If this matters in your workload,
//     a zero-alloc path can be introduced in a future version (see TODO).
//
// Usage
//   package mypkg
//   var (
//       FUserID    = xgxerror.Field[int64]("user_id")
//       FRequestID = xgxerror.Field[string]("request_id")
//   )
//
//   func do() xgxerror.Error {
//       err := xgxerror.NotFound("user", 42)
//       err = FUserID.Set(err, 42)
//       id, ok := FUserID.Get(err) // id=42, ok=true
//       _ = id; _ = ok
//       return err
//   }
//
// Caveats
//   • TypedField relies on Go’s type assertions. The dynamic type stored in the
//     error’s context MUST match T exactly; no implicit conversions are made.
//   • Set(nil, v) will create a NEW internal failure (same behavior as With(nil,...)).
//     If you do not want that, ensure you already have an Error before calling Set.
//
package xgxerror

import (
	"fmt"
)

// TypedField is a small, zero-policy helper for type-safe context access.
// T is the Go type you intend to store/retrieve for the given key.
type TypedField[T any] struct {
	key string
}

// Field constructs a TypedField[T] for a given key.
// Keys SHOULD be snake_case for consistency across logs/exports.
func Field[T any](key string) TypedField[T] {
	return TypedField[T]{key: key}
}

// Key returns the underlying string key for this field.
func (f TypedField[T]) Key() string { return f.key }

// Set attaches (key = val) to e and returns a NEW Error.
// If e is nil, Set behaves like With(nil, key, val): it creates a NEW internal
// failure carrying the field. If you do not want that, pass a non-nil Error.
func (f TypedField[T]) Set(e Error, val T) Error {
	// We intentionally go through the public adapter to handle nil safely and
	// preserve core semantics uniformly.
	return With(e, f.key, any(val))
}

// Get retrieves the typed value for this field from e.
// Returns (zero, false) if e is nil, the field is absent, or the value has a
// different dynamic type than T.
//
// NOTE: This performs a type assertion; aliases or convertible types are not
// accepted automatically — the stored dynamic type must match T exactly.
func (f TypedField[T]) Get(e Error) (T, bool) {
	var zero T
	if e == nil {
		return zero, false
	}
	m := e.Context() // copy-on-read (allocates a map)
	if m == nil {
		return zero, false
	}
	v, ok := m[f.key]
	if !ok {
		return zero, false
	}
	tv, ok := v.(T)
	if !ok {
		return zero, false
	}
	return tv, true
}

// MustGet retrieves the typed value or panics with a descriptive error if the
// field is missing or has a different dynamic type than T.
//
// Use sparingly — it is intended for test code or contexts where absence is a
// programming error rather than a runtime condition.
func (f TypedField[T]) MustGet(e Error) T {
	var zero T
	if e == nil {
		panic(fmt.Errorf("xgxerror.TypedField[%T](%q): error is nil", zero, f.key))
	}
	m := e.Context()
	v, ok := m[f.key]
	if !ok {
		panic(fmt.Errorf("xgxerror.TypedField[%T](%q): field missing", zero, f.key))
	}
	tv, ok := v.(T)
	if !ok {
		panic(fmt.Errorf("xgxerror.TypedField[%T](%q): wrong dynamic type (%T)", zero, f.key, v))
	}
	return tv
}

// TODO(v1.1):
//   • Provide an internal, zero-alloc iterator for xgx errors so Get/MustGet
//     can scan the underlying []Field without constructing a map. Keep the
//     public API identical; swap implementation when available.
