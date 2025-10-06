// typed_field.go — optional, type-safe field helpers for xgx-error core.
// SOTA: zero-alloc fast path for native xgx errors; safe fallback for foreign types.
//
// Copyright (c) 2025.
// SPDX-License-Identifier: MIT
//
// Overview
//
//	TypedField provides an *optional* ergonomic layer for attaching and reading
//	strongly-typed context fields on xgx errors. It does not replace the
//	plain string/any API (`With`, `Ctx`, `CtxBound`) — it complements it.
//
// Goals
//   - Zero policy: purely a convenience for authors who prefer typed access.
//   - No lock-in: you can mix `.With("k", any)` with `FieldOf[T]("k").Set/Get`.
//   - Interop-first: works over the public Error interface.
//   - Fast path: zero allocations when reading fields from native xgx errors.
//
// Implementation notes
//   - Get/MustGet first attempt a package-private fast path by asserting the
//     Error to an internal interface implemented by native xgx errors that
//     performs a direct lookup (no closures, no map allocs).
//   - If unavailable (foreign Error impl), we fall back to Error.Context() which
//     returns a copy (one map allocation).
//
// Usage
//
//	package mypkg
//	var (
//	    FUserID    = xgxerror.FieldOf[int64]("user_id")
//	    FRequestID = xgxerror.FieldOf[string]("request_id")
//	)
//
//	func do() xgxerror.Error {
//	    err := xgxerror.NotFound("user", 42)
//	    err = FUserID.Set(err, 42)
//	    id, ok := FUserID.Get(err) // id=42, ok=true
//	    _ = id; _ = ok
//	    return err
//	}
//
// Caveats
//   - TypedField relies on Go’s type assertions. The dynamic type stored in the
//     error’s context MUST match T exactly; no implicit conversions are made.
//   - Set(nil, v) will create a NEW internal failure (same behavior as With(nil,...)).
//     If you do not want that, ensure you already have an Error before calling Set.
package xgxerror

import "fmt"

// fieldLookup is a package-private capability implemented by native xgx errors
// (failureErr, defectErr, interruptErr). It must return the newest (last-write)
// value for the given key and whether it was found. No allocations.
type fieldLookup interface {
	lookupFieldLast(key string) (any, bool)
}

// TypedField is a small, zero-policy helper for type-safe context access.
// T is the Go type you intend to store/retrieve for the given key.
type TypedField[T any] struct {
	key string
}

// FieldOf constructs a TypedField[T] for a given key.
// Keys SHOULD be snake_case for consistency across logs/exports.
//
// Note: Named FieldOf to avoid collision with the package's Field struct in context.go.
func FieldOf[T any](key string) TypedField[T] {
	return TypedField[T]{key: key}
}

// Key returns the underlying string key for this field.
func (f TypedField[T]) Key() string { return f.key }

// Set attaches (key = val) to e and returns a NEW Error.
// If e is nil, Set behaves like With(nil, key, val): it creates a NEW internal
// failure carrying the field. If you do not want that, pass a non-nil Error.
func (f TypedField[T]) Set(e Error, val T) Error {
	// Route through public adapter to preserve nil behavior & semantics.
	return With(e, f.key, any(val))
}

// Get retrieves the typed value for this field from e.
// Returns (zero, false) if e is nil, the field is absent, or the value has a
// different dynamic type than T.
//
// Fast path: if e is a native xgx error, use fieldLookup (zero allocs, last-write-wins).
// Fallback: if not native, use e.Context() (allocates a map copy).
func (f TypedField[T]) Get(e Error) (T, bool) {
	var zero T
	if e == nil {
		return zero, false
	}

	// Zero-alloc fast path for native errors.
	if lk, ok := any(e).(fieldLookup); ok {
		val, hit := lk.lookupFieldLast(f.key)
		if !hit {
			return zero, false
		}
		tv, ok := val.(T)
		if !ok {
			return zero, false
		}
		return tv, true
	}

	// Fallback for foreign Error implementations.
	m := e.Context() // copy-on-read (allocates)
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

	// Fast path for native errors.
	if lk, ok := any(e).(fieldLookup); ok {
		val, hit := lk.lookupFieldLast(f.key)
		if !hit {
			panic(fmt.Errorf("xgxerror.TypedField[%T](%q): field missing", zero, f.key))
		}
		tv, ok := val.(T)
		if !ok {
			panic(fmt.Errorf("xgxerror.TypedField[%T](%q): wrong dynamic type (%T)", zero, f.key, val))
		}
		return tv
	}

	// Fallback for foreign Error implementations.
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
