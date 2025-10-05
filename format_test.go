package xgxerror

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// containsInOrder reports whether all needles appear in haystack in order.
func containsInOrder(haystack string, needles ...string) bool {
	pos := 0
	for _, n := range needles {
		i := strings.Index(haystack[pos:], n)
		if i < 0 {
			return false
		}
		pos += i + len(n)
	}
	return true
}

func TestFailureFormatting_ConciseAndVerbose(t *testing.T) {
	// Build a failure with message, code, context, and a captured stack.
	err := NotFound("user", 42).
		Ctx("lookup failed", "tenant", "acme", "attempt", 2).
		WithStack()

	// %v → concise, single-line-ish; must include message text.
	concise := fmt.Sprintf("%v", err)
	if !strings.Contains(concise, "lookup failed") {
		t.Fatalf("%%v missing message; got: %q", concise)
	}
	// The concise string for failureErr includes the code prefix when set.
	if !strings.Contains(concise, "not_found") {
		t.Fatalf("%%v should include code string; got: %q", concise)
	}

	// %+v → verbose, multi-line: code, msg, ctx, (stack present because WithStack()).
	verbose := fmt.Sprintf("%+v", err)
	wantFrags := []string{
		"code=not_found",
		`msg="lookup failed`,
		"\nctx:",
		" tenant=acme",
		" attempt=2",
		"\nstack:",
	}
	for _, w := range wantFrags {
		if !strings.Contains(verbose, w) {
			t.Fatalf("%%+v missing %q in:\n%s", w, verbose)
		}
	}

	// Context order is insertion order: tenant first, then attempt.
	if !containsInOrder(verbose, " ctx:", " tenant=acme", " attempt=2") {
		t.Fatalf("context order not preserved in verbose: %q", verbose)
	}
}

func TestDefectFormatting_VerboseIncludesStack(t *testing.T) {
	root := errors.New("boom")
	err := Defect(root) // defects capture stack at creation

	concise := fmt.Sprintf("%v", err)
	if !strings.Contains(concise, "defect") {
		t.Fatalf("%%v (defect) should contain 'defect'; got: %q", concise)
	}

	verbose := fmt.Sprintf("%+v", err)
	for _, w := range []string{"code=defect", "\nstack:"} {
		if !strings.Contains(verbose, w) {
			t.Fatalf("%%+v (defect) missing %q in:\n%s", w, verbose)
		}
	}
	// Cause should appear recursively on %+v.
	if !strings.Contains(verbose, "boom") {
		t.Fatalf("%%+v (defect) should include cause: %q", verbose)
	}
}

func TestInterruptFormatting_NoStack(t *testing.T) {
	err := Interrupt("client canceled").With("req_id", "r-1")
	concise := fmt.Sprintf("%v", err)
	if !strings.Contains(concise, "interrupt") {
		t.Fatalf("%%v (interrupt) should contain 'interrupt'; got: %q", concise)
	}
	verbose := fmt.Sprintf("%+v", err)
	// Interrupts have no stack; %+v must not print a stack section.
	if strings.Contains(verbose, "\nstack:") {
		t.Fatalf("interrupt %+v should not include stack; got:\n%s", verbose)
	}
	// But it must include code/msg/ctx.
	for _, w := range []string{"code=interrupt", `msg="client canceled"`, " ctx:", " req_id=r-1"} {
		if !strings.Contains(verbose, w) {
			t.Fatalf("interrupt %%+v missing %q in:\n%s", w, verbose)
		}
	}
}

func TestNestedCause_VerboseRecurses(t *testing.T) {
	cause := Invalid("email", "bad format")
	err := Internal(errors.Join(cause, errors.New("disk"))).Ctx("persist failed", "op", "write").WithStack()

	verbose := fmt.Sprintf("%+v", err)
	for _, w := range []string{
		"code=internal",
		`msg="persist failed`,
		"\nctx:",
		" op=write",
		"\ncause:",
		"invalid email",     // cause message path
		"code=invalid",      // cause code path (from %+v recursion)
	} {
		if !strings.Contains(verbose, w) {
			t.Fatalf("missing %q in verbose output:\n%s", w, verbose)
		}
	}
	// Expect a stack section for the top error (Internal.WithStack()).
	if !strings.Contains(verbose, "\nstack:") {
		t.Fatalf("top-level stack section missing in:\n%s", verbose)
	}
}

func TestFormat_JoinedLeavesAppearInDefaultString(t *testing.T) {
	a := NotFound("user", 1)
	b := Invalid("field", "oops")
	err := errors.Join(a, b)

	// The default %v of the join includes both child error strings;
	// we do not assert exact layout (stdlib-defined), only presence.
	s := fmt.Sprintf("%v", err)
	for _, leaf := range []string{a.Error(), b.Error()} {
		if !strings.Contains(s, leaf) {
			t.Fatalf("%%v(join) missing leaf %q in:\n%s", leaf, s)
		}
	}
}

// Formatting behavior rationale (not executed):
// - Implementing fmt.Formatter controls %v/%+v rendering. %+v commonly used
//   for verbose error detail (incl. stacks, nested causes). :contentReference[oaicite:1]{index=1}
// - errors.Join produces an error with Unwrap() []error; Is/As traverse both
//   single and multi unwraps; %+v recursion visits nested causes accordingly. :contentReference[oaicite:2]{index=2}
// - Stack frames are symbolized via runtime.CallersFrames, which accounts for
//   inlined functions; tests check for the presence of a "stack:" section, not
//   exact frame text (runtime-dependent). :contentReference[oaicite:3]{index=3}
