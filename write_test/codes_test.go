package xgxerror

import "testing"

func TestCodeIsBuiltin(t *testing.T) {
	for _, c := range AllBuiltinCodes {
		if !c.IsBuiltin() {
			t.Fatalf("expected %q to be builtin", c)
		}
	}

	nonBuiltin := Code("custom_code")
	if nonBuiltin.IsBuiltin() {
		t.Fatalf("expected %q to be non-builtin", nonBuiltin)
	}

	var empty Code
	if empty.IsBuiltin() {
		t.Fatalf("expected empty code to be non-builtin")
	}
}
