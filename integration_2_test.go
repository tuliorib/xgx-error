// integration_2_test.go — integration tests for TypedField fast/fallback paths,
// interop with existing errors, and basic benchmarks.
package xgxerror

import (
	"errors"
	"testing"
	"time"
)

//
// Foreign Error (no fast-path): a wrapper that implements Error but NOT fieldLookup.
//

type foreignErr struct {
	inner Error
}

func (f foreignErr) Error() string           { return f.inner.Error() }
func (f foreignErr) Unwrap() error           { return f.inner.Unwrap() }
func (f foreignErr) CodeVal() Code           { return f.inner.CodeVal() }
func (f foreignErr) Context() map[string]any { return f.inner.Context() }
func (f foreignErr) WithStack() Error        { return foreignErr{inner: f.inner.WithStack()} }
func (f foreignErr) WithStackSkip(skip int) Error {
	return foreignErr{inner: f.inner.WithStackSkip(skip)}
}
func (f foreignErr) Code(c Code) Error              { return foreignErr{inner: f.inner.Code(c)} }
func (f foreignErr) With(key string, val any) Error { return foreignErr{inner: f.inner.With(key, val)} }
func (f foreignErr) Ctx(msg string, kv ...any) Error {
	return foreignErr{inner: f.inner.Ctx(msg, kv...)}
}
func (f foreignErr) CtxBound(msg string, n int, kv ...any) Error {
	return foreignErr{inner: f.inner.CtxBound(msg, n, kv...)}
}
func (f foreignErr) MsgReplace(msg string) Error { return foreignErr{inner: f.inner.MsgReplace(msg)} }
func (f foreignErr) MsgAppend(msg string) Error  { return foreignErr{inner: f.inner.MsgAppend(msg)} }

func makeForeign(e Error) Error { return foreignErr{inner: e} }

//
// 5) Fallback Path (Foreign Errors)
//

func TestGet_FallsBackToContextMap(t *testing.T) {
	// No t.Parallel: we’ll measure allocations in this test.
	base := NotFound("obj", 1)
	fe := makeForeign(base)

	// Set via typed field — still returns a foreignErr
	fe = FieldOf[int]("k").Set(fe, 42)

	// Value present
	if v, ok := FieldOf[int]("k").Get(fe); !ok || v != 42 {
		t.Fatalf("foreign Get returned (v=%v ok=%v); want (42 true)", v, ok)
	}

	// Allocation check (fallback map copy expected → ≥ 1 alloc)
	field := FieldOf[int]("k")
	allocs := testing.AllocsPerRun(500, func() {
		_, _ = field.Get(fe)
	})
	if allocs < 1 {
		t.Fatalf("fallback path allocs=%v, want >=1 (map copy)", allocs)
	}
}

func TestGet_ForeignErrorWithEmptyContext(t *testing.T) {
	t.Parallel()

	fe := makeForeign(BadRequest("x")) // no fields
	if v, ok := FieldOf[string]("missing").Get(fe); ok || v != "" {
		t.Fatalf("expected (\"\",false); got (%q,%v)", v, ok)
	}
}

func TestMustGet_FallsBackForForeignErrors(t *testing.T) {
	// Panics checked; no need for t.Parallel or alloc checks here.
	t.Run("present", func(t *testing.T) {
		fe := makeForeign(Invalid("f", "bad"))
		fe = FieldOf[string]("s").Set(fe, "ok")
		if got := FieldOf[string]("s").MustGet(fe); got != "ok" {
			t.Fatalf("MustGet(foreign) got %q, want ok", got)
		}
	})
	t.Run("missing panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("MustGet missing should panic")
			}
		}()
		fe := makeForeign(BadRequest("x"))
		_ = FieldOf[int]("k").MustGet(fe)
	})
}

//
// 6) Integration with Existing Errors
//

func TestTypedField_WithSemanticConstructors(t *testing.T) {
	t.Parallel()

	// NotFound
	e := NotFound("doc", "a1")
	e = FieldOf[int]("n").Set(e, 7)
	if v, ok := FieldOf[int]("n").Get(e); !ok || v != 7 {
		t.Fatalf("NotFound+Set→Get failed")
	}

	// Invalid
	e = Invalid("name", "blank")
	e = FieldOf[string]("s").Set(e, "x")
	if v, ok := FieldOf[string]("s").Get(e); !ok || v != "x" {
		t.Fatalf("Invalid+Set→Get failed")
	}

	// Internal
	e = Internal(errors.New("boom"))
	e = FieldOf[bool]("flag").Set(e, true)
	if v, ok := FieldOf[bool]("flag").Get(e); !ok || !v {
		t.Fatalf("Internal+Set→Get failed")
	}
}

func TestTypedField_WithFluentAPI(t *testing.T) {
	t.Parallel()

	// .With then typed Get
	e := BadRequest("x").With("k", 123)
	if v, ok := FieldOf[int]("k").Get(e); !ok || v != 123 {
		t.Fatalf("With then Field.Get failed")
	}

	// typed Set then Context() map check
	e = FieldOf[string]("s").Set(e, "ok")
	if m := e.Context(); m["s"] != "ok" {
		t.Fatalf("typed Set not visible in Context map: %v", m)
	}
}

func TestTypedField_WithWrap(t *testing.T) {
	t.Parallel()

	plain := errors.New("plain")

	// From(plain) then Set → Get
	e := From(plain)
	e = FieldOf[int64]("id").Set(e, 99)
	if v, ok := FieldOf[int64]("id").Get(e); !ok || v != 99 {
		t.Fatalf("From+Set→Get failed")
	}

	// Wrap(plain, ...) then Set → Get
	e2 := Wrap(plain, "wrap")
	e2 = FieldOf[string]("note").Set(e2, "n1")
	if v, ok := FieldOf[string]("note").Get(e2); !ok || v != "n1" {
		t.Fatalf("Wrap+Set→Get failed")
	}
}

func TestTypedField_AfterMsgAppend(t *testing.T) {
	t.Parallel()

	e := Conflict("c")
	e = FieldOf[int]("k").Set(e, 1)
	e = e.MsgAppend("more context")
	if v, ok := FieldOf[int]("k").Get(e); !ok || v != 1 {
		t.Fatalf("field unreadable after MsgAppend")
	}
}

//
// 7) Performance (benchmarks + a dedicated alloc test)
//

func BenchmarkGet_NativeError_FastPath(b *testing.B) {
	e := FieldOf[int]("k").Set(BadRequest("x"), 42) // native type → fast path
	field := FieldOf[int]("k")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, ok := field.Get(e)
		if !ok || v != 42 {
			b.Fatalf("unexpected result: v=%v ok=%v", v, ok)
		}
	}
}

func BenchmarkGet_ForeignError_Fallback(b *testing.B) {
	e := FieldOf[int]("k").Set(makeForeign(BadRequest("x")), 42) // foreign → fallback
	field := FieldOf[int]("k")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, ok := field.Get(e)
		if !ok || v != 42 {
			b.Fatalf("unexpected result: v=%v ok=%v", v, ok)
		}
	}
}

func BenchmarkSet(b *testing.B) {
	field := FieldOf[int]("k")
	base := BadRequest("x")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = field.Set(base, i) // copy-on-write clone inside
	}
}

// Dedicated, non-parallel allocation assertion for native fast path.
// Ensures we do not regress to the fallback path accidentally.
func TestGet_NoAllocForNative(t *testing.T) {
	// No t.Parallel — allocation tests must run serially.
	e := FieldOf[int]("k").Set(BadRequest("x"), 123)
	field := FieldOf[int]("k")
	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = field.Get(e)
	})
	if allocs != 0 {
		t.Fatalf("native Get allocs=%v, want 0", allocs)
	}
}

// (Utility) ensure time-based typed fields behave too (smoke).
func TestTypedField_WithTimeoutDuration(t *testing.T) {
	t.Parallel()
	e := Timeout(250 * time.Millisecond)
	e = FieldOf[time.Duration]("d").Set(e, 2*time.Second)
	if v, ok := FieldOf[time.Duration]("d").Get(e); !ok || v != 2*time.Second {
		t.Fatalf("duration typed field failed: v=%v ok=%v", v, ok)
	}
}
