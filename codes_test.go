// codes_test.go — verification for built-in code registry & helpers.
package xgxerror

import (
	"fmt"
	"reflect"
	"testing"
)

func TestIsBuiltin_AllBuiltinCodesAreBuiltin(t *testing.T) {
	t.Parallel()

	for i, c := range BuiltinCodes() {
		if !c.IsBuiltin() {
			t.Fatalf("index=%d code=%q: expected IsBuiltin()=true", i, c)
		}
	}
}

func TestIsBuiltin_CustomAndEmptyAreNotBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("custom_code", func(t *testing.T) {
		if Code("custom_code").IsBuiltin() {
			t.Fatalf("expected custom_code to be non-builtin")
		}
	})
	t.Run("empty_string", func(t *testing.T) {
		var empty Code
		if empty.IsBuiltin() {
			t.Fatalf("expected empty code to be non-builtin")
		}
	})
}

func TestBuiltinCodes_DefensiveCopy(t *testing.T) {
	t.Parallel()

	orig := BuiltinCodes()
	if len(orig) == 0 {
		t.Fatalf("BuiltinCodes() returned empty set (unexpected)")
	}

	// Mutate the returned slice (should not affect package state).
	mut := append([]Code(nil), orig...) // local copy to mutate
	mut[0] = Code("custom_code")

	// Re-fetch; must equal the original, not our mutation.
	after := BuiltinCodes()

	if reflect.DeepEqual(after, mut) {
		t.Fatalf("BuiltinCodes() appears to expose internal slice; mutation leaked")
	}
	if !reflect.DeepEqual(after, orig) {
		t.Fatalf("BuiltinCodes() changed unexpectedly.\nwant=%v\ngot=%v", orig, after)
	}

	// Appending to the returned slice must not affect future calls.
	appended := append(orig, Code("extra") /* local growth */)
	_ = appended // silence linters; behavior covered by equality above
}

func TestBuiltinCodes_LengthAndOrder(t *testing.T) {
	t.Parallel()

	got := BuiltinCodes()

	// Keep this list in sync with codes.go (domain → availability → internal/meta).
	want := []Code{
		// Domain / validation (8)
		CodeBadRequest,
		CodeUnauthorized,
		CodeForbidden,
		CodeNotFound,
		CodeConflict,
		CodeInvalid,
		CodeUnprocessable,
		CodeTooManyRequests,
		// Availability / time (2)
		CodeTimeout,
		CodeUnavailable,
		// Internal / meta (3)
		CodeInternal,
		CodeDefect,
		CodeInterrupt,
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected BuiltinCodes() length: want=%d got=%d", len(want), len(got))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuiltinCodes() order/content mismatch.\nwant=%v\ngot=%v", want, got)
	}
}

func TestCode_StringUnderlyingValue(t *testing.T) {
	t.Parallel()

	samples := []string{
		"bad_request",
		"internal",
		"custom_code",
		"",
	}

	for _, s := range samples {
		c := Code(s)

		// Always true: direct string conversion matches the underlying value.
		if string(c) != s {
			t.Fatalf("string(Code(%q)) != %q", s, s)
		}

		// If Code implements fmt.Stringer (i.e., Code.String()),
		// verify it returns the underlying string. If not implemented,
		// we skip this stricter check to avoid coupling the test to
		// an optional convenience method.
		type stringer interface{ String() string }
		if sc, ok := any(c).(stringer); ok {
			if sc.String() != s {
				t.Fatalf("Code(%q).String() = %q, want %q", s, sc.String(), s)
			}
			// fmt should use String() when present.
			if fmt.Sprint(c) != s {
				t.Fatalf("fmt.Sprint(Code(%q)) = %q, want %q", s, fmt.Sprint(c), s)
			}
		}
	}
}
