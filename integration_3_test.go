// integration_3_test.go — edge cases, concurrency, error messages, CtxBound interop,
// and documentation example tests for TypedField and xgx-error interop.
package xgxerror

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

//
// 8) Edge Cases
//

func TestTypedField_EmptyKey(t *testing.T) {
	t.Parallel()

	// Empty keys are allowed in the internal []Field, but filtered out from Context() maps.
	e := FieldOf[int]("").Set(BadRequest("x"), 123)

	// TypedField fast path should still retrieve it (works).
	if v, ok := FieldOf[int]("").Get(e); !ok || v != 123 {
		t.Fatalf("empty key typed get failed: v=%v ok=%v", v, ok)
	}

	// Context() must filter empty-string keys (see context.go semantics).
	if _, ok := e.Context()[""]; ok {
		t.Fatalf("Context map must filter empty-string keys")
	}
}

func TestTypedField_DuplicateKeys_LastWins(t *testing.T) {
	t.Parallel()

	// Native (fast path)
	e := FieldOf[int]("k").Set(BadRequest("x"), 1)
	e = FieldOf[int]("k").Set(e, 2)
	if v, ok := FieldOf[int]("k").Get(e); !ok || v != 2 {
		t.Fatalf("last-write-wins failed (native): v=%v ok=%v", v, ok)
	}

	// Foreign (fallback path) should also honor last-write-wins
	fe := FieldOf[int]("k").Set(makeForeign(BadRequest("x")), 10)
	fe = FieldOf[int]("k").Set(fe, 11)
	if v, ok := FieldOf[int]("k").Get(fe); !ok || v != 11 {
		t.Fatalf("last-write-wins failed (foreign): v=%v ok=%v", v, ok)
	}
}

func TestTypedField_MultipleTypedFieldsSameKey(t *testing.T) {
	t.Parallel()

	e := FieldOf[int]("k").Set(BadRequest("x"), 5)
	if v, ok := FieldOf[string]("k").Get(e); ok || v != "" {
		t.Fatalf("Get[string] on int field should be (\"\",false); got (%q,%v)", v, ok)
	}
}

func TestTypedField_PointerTypes(t *testing.T) {
	t.Parallel()

	type User struct{ ID int }

	// Non-nil pointer
	u := &User{ID: 7}
	e := FieldOf[*User]("user").Set(BadRequest("x"), u)
	got, ok := FieldOf[*User]("user").Get(e)
	if !ok || got == nil || got.ID != 7 || got != u {
		t.Fatalf("pointer roundtrip failed: got=%v ok=%v (identity=%v)", got, ok, got == u)
	}

	// Nil pointer value
	e = FieldOf[*User]("nilp").Set(e, nil)
	got2, ok2 := FieldOf[*User]("nilp").Get(e)
	if !ok2 || got2 != nil {
		t.Fatalf("nil pointer roundtrip failed: got=%v ok=%v", got2, ok2)
	}
}

func TestTypedField_ZeroValues(t *testing.T) {
	t.Parallel()

	// Present-but-zero should be (zero, true)
	e := FieldOf[int]("i0").Set(BadRequest("x"), 0)
	if v, ok := FieldOf[int]("i0").Get(e); !ok || v != 0 {
		t.Fatalf("present-zero int must be (0,true); got (%v,%v)", v, ok)
	}

	e = FieldOf[string]("s0").Set(e, "")
	if v, ok := FieldOf[string]("s0").Get(e); !ok || v != "" {
		t.Fatalf("present-zero string must be (\"\",true); got (%q,%v)", v, ok)
	}
}

func TestTypedField_LargeValues(t *testing.T) {
	t.Parallel()

	blob := make([]byte, 1<<20) // 1MB
	for i := range blob {
		blob[i] = byte(i)
	}
	e := FieldOf[[]byte]("b").Set(BadRequest("x"), blob)
	got, ok := FieldOf[[]byte]("b").Get(e)
	if !ok || len(got) != len(blob) || got[123456] != blob[123456] {
		t.Fatalf("large slice roundtrip failed: ok=%v len=%d", ok, len(got))
	}

	var arr [100]int
	for i := range arr {
		arr[i] = i
	}
	e = FieldOf[[100]int]("arr").Set(e, arr)
	gotArr, ok := FieldOf[[100]int]("arr").Get(e)
	if !ok || gotArr[99] != 99 {
		t.Fatalf("large array roundtrip failed: ok=%v last=%d", ok, gotArr[99])
	}
}

//
// 9) Concurrency
//

func TestTypedField_ConcurrentReads(t *testing.T) {
	t.Parallel()

	e := FieldOf[int]("k").Set(BadRequest("x"), 777)

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	errCh := make(chan error, N)

	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			v, ok := FieldOf[int]("k").Get(e)
			if !ok || v != 777 {
				errCh <- fmt.Errorf("got (%v,%v), want (777,true)", v, ok)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestTypedField_ConcurrentSetCreatesNewErrors(t *testing.T) {
	t.Parallel()

	base := BadRequest("x")
	const N = 64

	var wg sync.WaitGroup
	out := make([]Error, N)
	wg.Add(N)

	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			out[i] = FieldOf[int]("k").Set(base, i)
		}()
	}
	wg.Wait()

	// Original unchanged
	if _, ok := base.Context()["k"]; ok {
		t.Fatalf("base mutated; ctx=%v", base.Context())
	}

	// Each result contains its own value
	for i := 0; i < N; i++ {
		v, ok := FieldOf[int]("k").Get(out[i])
		if !ok || v != i {
			t.Fatalf("derived[%d] missing or wrong value: v=%v ok=%v", i, v, ok)
		}
	}
}

//
// 10) Error Messages
//

func TestMustGet_PanicMessage_Missing(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for missing field")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "field missing") || !strings.Contains(msg, "user_id") {
			t.Fatalf("panic message not descriptive: %q", msg)
		}
	}()
	e := BadRequest("x")
	_ = FieldOf[int64]("user_id").MustGet(e)
}

func TestMustGet_PanicMessage_WrongType(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for wrong type")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "wrong dynamic type") || !strings.Contains(msg, "k") {
			t.Fatalf("panic message not descriptive: %q", msg)
		}
	}()
	e := FieldOf[string]("k").Set(BadRequest("x"), "s")
	_ = FieldOf[int]("k").MustGet(e)
}

func TestMustGet_PanicMessage_NilError(t *testing.T) {
	// Do not parallelize panic+recover path unnecessarily.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for nil error")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "error is nil") {
			t.Fatalf("panic message not descriptive for nil error: %q", msg)
		}
	}()
	_ = FieldOf[int]("k").MustGet(nil)
}

//
// 11) Interop with CtxBound
//

func TestTypedField_AfterCtxBound_Truncation(t *testing.T) {
	t.Parallel()

	// Build many fields, then bound to 5 newest.
	e := BadRequest("x")
	for i := 0; i < 20; i++ {
		e = FieldOf[int](fmt.Sprintf("k%d", i)).Set(e, i)
	}
	e = e.CtxBound("", 5) // keep newest 5 → k15..k19

	for i := 0; i < 15; i++ {
		if _, ok := FieldOf[int](fmt.Sprintf("k%d", i)).Get(e); ok {
			t.Fatalf("old key k%d should have been truncated", i)
		}
	}
	for i := 15; i < 20; i++ {
		if v, ok := FieldOf[int](fmt.Sprintf("k%d", i)).Get(e); !ok || v != i {
			t.Fatalf("newest key k%d missing: v=%v ok=%v", i, v, ok)
		}
	}
}

func TestTypedField_CtxBound_PreservesTypeSafety(t *testing.T) {
	t.Parallel()

	e := FieldOf[int]("n").Set(BadRequest("x"), 42)
	e = e.CtxBound("", 10)
	if v, ok := FieldOf[int]("n").Get(e); !ok || v != 42 {
		t.Fatalf("typed field lost across CtxBound")
	}
	if v, ok := FieldOf[string]("n").Get(e); ok || v != "" {
		t.Fatalf("type safety violated after CtxBound: got (%q,%v)", v, ok)
	}
}

//
// 12) Documentation Examples
//

func TestTypedField_ReadmeExample(t *testing.T) {
	t.Parallel()

	// From docs in typed_field.go Overview/Usage.
	type local = int64 // to emphasize explicit type
	var (
		FUserID    = FieldOf[local]("user_id")
		FRequestID = FieldOf[string]("request_id")
	)

	err := NotFound("user", 42)
	err = FUserID.Set(err, 42)
	err = FRequestID.Set(err, "r-123")

	id, ok := FUserID.Get(err)
	if !ok || id != 42 {
		t.Fatalf("FUserID.Get failed: id=%v ok=%v", id, ok)
	}
	rid, ok := FRequestID.Get(err)
	if !ok || rid != "r-123" {
		t.Fatalf("FRequestID.Get failed: rid=%v ok=%v", rid, ok)
	}
}

func TestTypedField_RegistryPattern(t *testing.T) {
	t.Parallel()

	// Define app-level registry
	var (
		UserID    = FieldOf[int64]("user_id")
		RequestID = FieldOf[string]("request_id")
	)

	// Use in multiple places
	e := Conflict("duplicate").With("route", "/users")
	e = UserID.Set(e, 1001)
	e = RequestID.Set(e, "req-xyz")

	// Verify consistency
	if uid, ok := UserID.Get(e); !ok || uid != 1001 {
		t.Fatalf("UserID.Get mismatch: %v %v", uid, ok)
	}
	if rid, ok := RequestID.Get(e); !ok || rid != "req-xyz" {
		t.Fatalf("RequestID.Get mismatch: %v %v", rid, ok)
	}
	if e.Context()["route"] != "/users" {
		t.Fatalf("existing untyped field lost: %v", e.Context())
	}
}
