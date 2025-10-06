// context_test.go — verification of context slice/map semantics.
package xgxerror

import (
	"reflect"
	"testing"
)

func TestCtxFromKV_EmptyInputReturnsEmptyFields(t *testing.T) {
	t.Parallel()

	fs := ctxFromKV()
	if len(fs) != 0 {
		t.Fatalf("expected empty fields, got len=%d", len(fs))
	}
	// Must be the canonical empty slice or at least empty (identity not required).
	if !reflect.DeepEqual(fs, emptyFields) {
		t.Fatalf("expected canonical emptyFields; got %#v", fs)
	}
}

func TestCtxFromKV_ValidPairsPreserveOrder(t *testing.T) {
	t.Parallel()

	fs := ctxFromKV("k1", 1, "k2", 2, "k3", 3)
	want := fields{{Key: "k1", Val: 1}, {Key: "k2", Val: 2}, {Key: "k3", Val: 3}}

	if !reflect.DeepEqual(fs, want) {
		t.Fatalf("order mismatch.\nwant=%#v\ngot =%#v", want, fs)
	}
}

func TestCtxFromKV_NonStringKeyDropsEntirePair(t *testing.T) {
	t.Parallel()

	fs := ctxFromKV(123, "v1", "k2", "v2")
	want := fields{{Key: "k2", Val: "v2"}}

	if !reflect.DeepEqual(fs, want) {
		t.Fatalf("non-string key should drop whole pair.\nwant=%#v\ngot =%#v", want, fs)
	}
}

func TestCtxFromKV_TrailingKeyBecomesNilPair(t *testing.T) {
	t.Parallel()

	fs := ctxFromKV("k1", 1, "lonely")
	if len(fs) != 2 {
		t.Fatalf("expected 2 fields, got %d: %#v", len(fs), fs)
	}
	if fs[1].Key != "lonely" || fs[1].Val != nil {
		t.Fatalf("expected trailing key => (key,nil); got %#v", fs[1])
	}
}

func TestCtxFromKV_MixedValidInvalidAlignsCorrectly(t *testing.T) {
	t.Parallel()

	// "a",1, 123,"x", "b",2   => drop (123,"x")
	fs := ctxFromKV("a", 1, 123, "x", "b", 2)
	want := fields{{Key: "a", Val: 1}, {Key: "b", Val: 2}}

	if !reflect.DeepEqual(fs, want) {
		t.Fatalf("alignment broken.\nwant=%#v\ngot =%#v", want, fs)
	}
}

func TestCtxCloneAppend_EmptyAddReturnsDstDirectly_NoAllocWhenPossible(t *testing.T) {
	t.Parallel()

	// Non-empty dst: should return the same slice header/backing (no copy).
	dst := fields{{Key: "k1", Val: 1}, {Key: "k2", Val: 2}}
	got := ctxCloneAppend(dst /* add empty */)

	if len(got) != len(dst) {
		t.Fatalf("length changed: want=%d got=%d", len(dst), len(got))
	}
	// Same backing array pointer (len>0 is guaranteed here)
	if &got[0] != &dst[0] {
		t.Fatalf("expected no new allocation for no-op append (same backing)")
	}

	// Empty dst: should return canonical emptyFields (identity not required).
	var dstEmpty fields
	gotEmpty := ctxCloneAppend(dstEmpty /* add empty */)
	if len(gotEmpty) != 0 {
		t.Fatalf("expected empty slice, got len=%d", len(gotEmpty))
	}
}

func TestCtxCloneAppend_NonEmptyAddAllocatesFreshBacking(t *testing.T) {
	t.Parallel()

	dst := fields{{Key: "k1", Val: 1}}
	add := []Field{{Key: "k2", Val: 2}}

	got := ctxCloneAppend(dst, add...)
	if len(got) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(got))
	}
	// Different backing (avoid aliasing on append)
	if &got[0] == &dst[0] {
		t.Fatalf("expected fresh allocation (different backing array)")
	}
}

func TestCtxCloneAppend_NoAliasingOnReturnedSlice(t *testing.T) {
	t.Parallel()

	dst := fields{{Key: "k1", Val: 1}}
	add := []Field{{Key: "k2", Val: 2}}
	got := ctxCloneAppend(dst, add...)

	// Mutate returned slice; original must remain unchanged.
	got[0].Val = 999
	if dst[0].Val.(int) != 1 {
		t.Fatalf("aliasing detected: dst mutated after modifying returned slice")
	}
}

func TestCtxToMap_AlwaysNonNil(t *testing.T) {
	t.Parallel()

	m := ctxToMap(emptyFields)
	if m == nil {
		t.Fatalf("ctxToMap must return non-nil map")
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map for empty fields, got len=%d", len(m))
	}
}

func TestCtxToMap_FiltersEmptyKeys(t *testing.T) {
	t.Parallel()

	fs := fields{
		{Key: "", Val: "drop-me"},
		{Key: "k", Val: "v"},
	}
	m := ctxToMap(fs)

	if _, ok := m[""]; ok {
		t.Fatalf("ctxToMap must filter empty-string keys")
	}
	if v, ok := m["k"]; !ok || v != "v" {
		t.Fatalf("expected k=v to remain; got %v (ok=%v)", v, ok)
	}
}

func TestCtxToMap_LastWriteWinsForDuplicates(t *testing.T) {
	t.Parallel()

	fs := fields{
		{Key: "dup", Val: 1},
		{Key: "dup", Val: 2},
		{Key: "dup", Val: 3},
	}
	m := ctxToMap(fs)

	if len(m) != 1 {
		t.Fatalf("expected 1 key after duplicate collapse, got %d", len(m))
	}
	if m["dup"] != 3 {
		t.Fatalf("last-write-wins violated: want dup=3, got %v", m["dup"])
	}
}

func TestCtxToMap_DefensiveCopy(t *testing.T) {
	t.Parallel()

	fs := fields{
		{Key: "a", Val: 1},
		{Key: "b", Val: 2},
	}
	m1 := ctxToMap(fs)
	// Mutate m1; calling ctxToMap again must not be affected.
	m1["a"] = 999
	delete(m1, "b")

	m2 := ctxToMap(fs)

	// Original field-derived values should be present again.
	if m2["a"] != 1 || m2["b"] != 2 {
		t.Fatalf("ctxToMap did not return a fresh copy; got m2=%v", m2)
	}
	if reflect.DeepEqual(m1, m2) {
		// They may coincidentally equal for some inputs; we force a difference above.
		t.Fatalf("expected m1 and m2 to differ after mutating m1; m1=%v m2=%v", m1, m2)
	}
}
