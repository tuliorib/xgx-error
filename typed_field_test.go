// typed_field_1_test.go — basic, type-safety, nil-handling, and fast-path tests for typed_field.go.
package xgxerror

import (
	"errors"
	"io"
	"strings"
	"testing"
)

//
// 1) Basic Functionality
//

func TestFieldOf_Constructor(t *testing.T) {
	t.Parallel()

	tfI := FieldOf[int]("i")
	if tfI.Key() != "i" {
		t.Fatalf("FieldOf[int](\"i\").Key() = %q, want %q", tfI.Key(), "i")
	}

	tfS := FieldOf[string]("s")
	if tfS.Key() != "s" {
		t.Fatalf("FieldOf[string](\"s\").Key() = %q, want %q", tfS.Key(), "s")
	}

	type U struct{ A, B int }
	tfU := FieldOf[U]("u")
	if tfU.Key() != "u" {
		t.Fatalf("FieldOf[U](\"u\").Key() = %q, want %q", tfU.Key(), "u")
	}
}

func TestSet_BasicRoundTrip(t *testing.T) {
	t.Parallel()

	type S struct{ X int }
	type P struct{ S string }

	base := NotFound("obj", 1) // native xgx error (fast path)

	cases := []struct {
		name string
		set  func(Error) Error
		get  func(Error) (bool, string)
	}{
		{"int", func(e Error) Error {
			return FieldOf[int]("k").Set(e, 7)
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[int]("k").Get(e)
			return ok && v == 7, "int=7"
		}},
		{"int64", func(e Error) Error {
			return FieldOf[int64]("k64").Set(e, int64(42))
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[int64]("k64").Get(e)
			return ok && v == 42, "int64=42"
		}},
		{"string", func(e Error) Error {
			return FieldOf[string]("s").Set(e, "hello")
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[string]("s").Get(e)
			return ok && v == "hello", "string=hello"
		}},
		{"bool", func(e Error) Error {
			return FieldOf[bool]("b").Set(e, true)
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[bool]("b").Get(e)
			return ok && v, "bool=true"
		}},
		{"float64", func(e Error) Error {
			return FieldOf[float64]("f").Set(e, 3.14)
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[float64]("f").Get(e)
			return ok && v == 3.14, "float64=3.14"
		}},
		{"struct", func(e Error) Error {
			return FieldOf[S]("st").Set(e, S{X: 9})
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[S]("st").Get(e)
			return ok && v.X == 9, "struct{X=9}"
		}},
		{"slice", func(e Error) Error {
			return FieldOf[[]int]("sl").Set(e, []int{1, 2, 3})
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[[]int]("sl").Get(e)
			return ok && len(v) == 3 && v[2] == 3, "slice=[1,2,3]"
		}},
		{"map", func(e Error) Error {
			return FieldOf[map[string]int]("m").Set(e, map[string]int{"a": 1})
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[map[string]int]("m").Get(e)
			return ok && v["a"] == 1, "map[a]=1"
		}},
		{"interface-any", func(e Error) Error {
			return FieldOf[any]("any").Set(e, P{S: "p"})
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[any]("any").Get(e)
			p, ok2 := v.(P)
			return ok && ok2 && p.S == "p", "any(P{S:p})"
		}},
		{"pointer", func(e Error) Error {
			p := &P{S: "ptr"}
			return FieldOf[*P]("ptr").Set(e, p)
		}, func(e Error) (bool, string) {
			v, ok := FieldOf[*P]("ptr").Get(e)
			return ok && v != nil && v.S == "ptr", "*P{S:ptr}"
		}},
	}

	e := base
	for _, tc := range cases {
		e = tc.set(e)
		if ok, hint := tc.get(e); !ok {
			t.Fatalf("%s roundtrip failed (%s)", tc.name, hint)
		}
	}
}

func TestGet_ReturnsZeroWhenAbsent(t *testing.T) {
	t.Parallel()

	e := BadRequest("x")
	if v, ok := FieldOf[int]("i").Get(e); ok || v != 0 {
		t.Fatalf("absent int: got (v=%v, ok=%v), want (0,false)", v, ok)
	}
	if v, ok := FieldOf[string]("s").Get(e); ok || v != "" {
		t.Fatalf("absent string: got (v=%q, ok=%v), want (\"\",false)", v, ok)
	}
	type S struct{}
	if v, ok := FieldOf[*S]("p").Get(e); ok || v != nil {
		t.Fatalf("absent *S: got (v=%v, ok=%v), want (nil,false)", v, ok)
	}
	if v, ok := FieldOf[bool]("b").Get(e); ok || v {
		t.Fatalf("absent bool: got (v=%v, ok=%v), want (false,false)", v, ok)
	}
}

func TestMustGet_ReturnsValueWhenPresent(t *testing.T) {
	t.Parallel()

	e := FieldOf[int]("k").Set(BadRequest("x"), 99)
	v := FieldOf[int]("k").MustGet(e)
	if v != 99 {
		t.Fatalf("MustGet returned %v, want 99", v)
	}
}

//
// 2) Type Safety
//

func TestGet_TypeMismatch(t *testing.T) {
	t.Parallel()

	e := FieldOf[string]("k").Set(BadRequest("x"), "s")
	if v, ok := FieldOf[int]("k").Get(e); ok || v != 0 {
		t.Fatalf("mismatch int<-string: got (v=%v, ok=%v), want (0,false)", v, ok)
	}

	e = FieldOf[int]("k2").Set(e, 7)
	if v, ok := FieldOf[string]("k2").Get(e); ok || v != "" {
		t.Fatalf("mismatch string<-int: got (v=%q, ok=%v), want (\"\",false)", v, ok)
	}
}

func TestMustGet_PanicOnTypeMismatch(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("MustGet should panic on type mismatch")
		}
	}()
	e := FieldOf[string]("k").Set(BadRequest("x"), "s")
	_ = FieldOf[int]("k").MustGet(e)
}

func TestSet_PreservesExactType(t *testing.T) {
	t.Parallel()

	e := FieldOf[int64]("k64").Set(BadRequest("x"), int64(42))
	if v, ok := FieldOf[int64]("k64").Get(e); !ok || v != 42 {
		t.Fatalf("Get[int64] failed: v=%v ok=%v", v, ok)
	}
	if v, ok := FieldOf[int]("k64").Get(e); ok || v != 0 {
		t.Fatalf("Get[int] should fail on int64 value: v=%v ok=%v", v, ok)
	}
}

func TestTypedField_InterfaceValues(t *testing.T) {
	t.Parallel()

	// any
	e := FieldOf[any]("x").Set(BadRequest("x"), errors.New("boom"))
	if v, ok := FieldOf[any]("x").Get(e); !ok || v == nil {
		t.Fatalf("Field[any] failed to store error value")
	}

	// error
	e = FieldOf[error]("err").Set(e, io.EOF)
	if v, ok := FieldOf[error]("err").Get(e); !ok || v != io.EOF {
		t.Fatalf("Field[error] roundtrip failed; got %v, ok=%v", v, ok)
	}

	// io.Reader
	r := strings.NewReader("hi")
	e = FieldOf[io.Reader]("r").Set(e, r)
	if v, ok := FieldOf[io.Reader]("r").Get(e); !ok || v == nil {
		t.Fatalf("Field[io.Reader] roundtrip failed; ok=%v v=%v", ok, v)
	}
}

//
// 3) Nil Handling
//

func TestSet_NilError(t *testing.T) {
	t.Parallel()

	// Set(nil, val) should create a new internal failure carrying the field.
	e := FieldOf[int]("k").Set(nil, 5)
	if v, ok := FieldOf[int]("k").Get(e); !ok || v != 5 {
		t.Fatalf("Set(nil,5) then Get failed: v=%v ok=%v", v, ok)
	}
}

func TestGet_NilError(t *testing.T) {
	t.Parallel()

	if v, ok := FieldOf[string]("k").Get(nil); ok || v != "" {
		t.Fatalf("Get(nil) should return (\"\",false); got (%q,%v)", v, ok)
	}
}

func TestMustGet_NilError(t *testing.T) {
	// NOTE: This test MUST NOT use t.Parallel(), as it doesn't benefit and we keep
	// panic path simple and deterministic.
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("MustGet(nil) should panic")
		}
	}()
	_ = FieldOf[int]("k").MustGet(nil)
}

//
// 4) Fast Path (Native xgx Errors) — allocation-sensitive tests MUST NOT be parallel
//

func TestGet_UsesIteratorForNativeErrors(t *testing.T) {
	// No t.Parallel here — testing.AllocsPerRun requires a non-parallel test.

	t.Run("failureErr", func(t *testing.T) {
		e := FieldOf[int]("k").Set(BadRequest("x"), 1)
		field := FieldOf[int]("k") // hoist to avoid measuring setup allocs
		allocs := testing.AllocsPerRun(1000, func() {
			_, _ = field.Get(e)
		})
		if allocs != 0 {
			t.Fatalf("fast path (failureErr) allocs=%v, want 0", allocs)
		}
	})

	t.Run("defectErr", func(t *testing.T) {
		e := FieldOf[int]("k").Set(Defect(errors.New("boom")), 2)
		field := FieldOf[int]("k")
		allocs := testing.AllocsPerRun(1000, func() {
			_, _ = field.Get(e)
		})
		if allocs != 0 {
			t.Fatalf("fast path (defectErr) allocs=%v, want 0", allocs)
		}
	})

	t.Run("interruptErr", func(t *testing.T) {
		e := FieldOf[int]("k").Set(Interrupt("stop"), 3)
		field := FieldOf[int]("k")
		allocs := testing.AllocsPerRun(1000, func() {
			_, _ = field.Get(e)
		})
		if allocs != 0 {
			t.Fatalf("fast path (interruptErr) allocs=%v, want 0", allocs)
		}
	})
}

func TestGet_LastWriteWins_FastPath(t *testing.T) {
	t.Parallel()

	base := BadRequest("x")
	e := FieldOf[int]("k").Set(base, 1)
	e = FieldOf[int]("k").Set(e, 2) // newer write should win

	v, ok := FieldOf[int]("k").Get(e)
	if !ok || v != 2 {
		t.Fatalf("last-write-wins failed; got (v=%v ok=%v), want (2,true)", v, ok)
	}
}

func TestMustGet_UsesIteratorForNativeErrors(t *testing.T) {
	// No t.Parallel here — testing.AllocsPerRun requires a non-parallel test.

	e := FieldOf[int]("k").Set(BadRequest("x"), 77)
	field := FieldOf[int]("k")
	allocs := testing.AllocsPerRun(1000, func() {
		_ = field.MustGet(e)
	})
	if allocs != 0 {
		t.Fatalf("MustGet fast path allocs=%v, want 0", allocs)
	}
}
