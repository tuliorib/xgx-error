// format_test.go — verification of fmt.Formatter implementations and sections.
package xgxerror

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func containsAll(t *testing.T, s string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			t.Fatalf("expected output to contain %q\n--- got ---\n%s", sub, s)
		}
	}
}

func notContains(t *testing.T, s string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			t.Fatalf("expected output to NOT contain %q\n--- got ---\n%s", sub, s)
		}
	}
}

func TestPercentV_ConciseIncludesMessage(t *testing.T) {
	t.Parallel()

	e := Conflict("duplicate email")
	out := fmt.Sprintf("%v", e)

	// concise form is error string; must include code prefix text and message
	containsAll(t, out, "conflict", "duplicate email")
}

func TestPercentV_IncludesCodePrefixWhenSet(t *testing.T) {
	t.Parallel()

	e := BadRequest("bad thing")
	out := fmt.Sprintf("%v", e)
	// Default string for failures shows "bad_request: bad thing"
	containsAll(t, out, "bad_request:", "bad thing")
}

func TestPercentPlusV_IncludesAllSections(t *testing.T) {
	t.Parallel()

	cause := errors.New("db timeout")
	e := Internal(cause).Ctx("", "service", "users-db", "attempt", 3).WithStack()
	out := fmt.Sprintf("%+v", e)

	// Sections in %+v:
	//  code=internal
	//  msg="internal error"
	//  ctx: ...
	//  cause: <default-string-of-cause or nested %+v if formatter-aware>
	//  stack: ...
	containsAll(t, out,
		"code=internal",
		`msg="internal error"`,
		"ctx:", "service=", "users-db", "attempt=", "3",
		"cause:", "db timeout",
		"stack:",
	)
}

func TestPercentPlusV_ContextPreservesInsertionOrder(t *testing.T) {
	t.Parallel()

	e := BadRequest("x").Ctx("", "k1", 1, "k2", 2, "k3", 3)
	out := fmt.Sprintf("%+v", e)

	// Ensure text order in rendered ctx is insertion order.
	k1 := strings.Index(out, "k1=")
	k2 := strings.Index(out, "k2=")
	k3 := strings.Index(out, "k3=")
	if k1 == -1 || k2 == -1 || k3 == -1 {
		t.Fatalf("context keys not found in %%+v output:\n%s", out)
	}
	if !(k1 < k2 && k2 < k3) {
		t.Fatalf("context order not preserved (k1<k2<k3). positions: k1=%d k2=%d k3=%d\n%s", k1, k2, k3, out)
	}
}

func TestPercentPlusV_RecursivelyFormatsCauseWithPlusV(t *testing.T) {
	t.Parallel()

	// Wrap a Defect (which has its own stack) as the cause of an Internal
	root := Defect(fmt.Errorf("broken invariant")).Ctx("root", "a", 1)
	top := Internal(root).Ctx("wrap", "b", 2).WithStack()

	out := fmt.Sprintf("%+v", top)

	// In the cause section, we should see the nested defect summary
	// in structured form ("code=defect msg="root" ...").
	containsAll(t, out,
		"code=internal", `msg="internal error"`,
		"ctx:", "b=", "2", // outer ctx
		"cause:",
		"code=defect", `msg="root"`, // nested defect header
		"ctx:", "a=", "1",           // nested ctx
		"broken invariant",          // nested cause default string
		"stack:",                    // at least one stack (inner or outer)
	)
}

func TestPercentPlusV_OmitsStackSectionWhenNotPresent(t *testing.T) {
	t.Parallel()

	// No stack captured here.
	e := BadRequest("bad").Ctx("", "k", "v")
	out := fmt.Sprintf("%+v", e)

	notContains(t, out, "\nstack:")
}

func TestPercentS_EqualsPercentV(t *testing.T) {
	t.Parallel()

	e := Unavailable("auth")
	v := fmt.Sprintf("%v", e)
	s := fmt.Sprintf("%s", e)
	if s != v {
		t.Fatalf("%%s should equal %%v.\n%%v=%q\n%%s=%q", v, s)
	}
}

func TestPercentQ_QuotesTheErrorString(t *testing.T) {
	t.Parallel()

	e := Timeout(250 * time.Millisecond) // Error string like "timeout"
	v := fmt.Sprintf("%v", e)
	q := fmt.Sprintf("%q", e)
	if q != strconv.Quote(v) {
		t.Fatalf("%%q must equal quoted %%v.\n%%v=%q\n%%q=%q", v, q)
	}
}

func TestDefectPercentPlusV_NoDuplicateHeader(t *testing.T) {
	t.Parallel()

	e := Defect(fmt.Errorf("x"))
	out := fmt.Sprintf("%+v", e)

	// We expect exactly one top-level defect header line.
	// Count "code=defect" occurrences: standalone defect should have 1.
	if c := strings.Count(out, "code=defect"); c != 1 {
		t.Fatalf("expected exactly one 'code=defect' header, got %d\n--- out ---\n%s", c, out)
	}
	containsAll(t, out, `msg="x"`, "cause:", "stack:")
}

func TestInterruptPercentPlusV_IncludesCause_NoStackSection(t *testing.T) {
	t.Parallel()

	e := Interrupt("stop now")
	out := fmt.Sprintf("%+v", e)

	// Structured header + cause section present; no stack section for interrupts.
	containsAll(t, out, "code=interrupt", `msg="stop now"`, "cause:")
	notContains(t, out, "\nstack:")
}

func TestJoinedErrors_ShowAllLeavesInDefaultAndVerboseFormat(t *testing.T) {
	t.Parallel()

	e1 := Conflict("c1").Ctx("", "k1", 1)
	e2 := Invalid("name", "blank")
	joined := errors.Join(e1, e2)

	out := fmt.Sprintf("%v", joined)
	// Default format should include both leaves' messages.
	containsAll(t, out, "conflict: c1", "invalid name")

	// For %+v, ensure both appear as well (at least their default strings).
	outPlus := fmt.Sprintf("%+v", joined)
	containsAll(t, outPlus, "conflict: c1", "invalid name")
}

func TestPercentPlusV_MinimalDefectSections(t *testing.T) {
	t.Parallel()

	// Defect already has a captured stack at creation; ensure %+v prints sections.
	e := Defect(fmt.Errorf("boom"))
	out := fmt.Sprintf("%+v", e)

	containsAll(t, out, "code=defect", `msg="boom"`, "cause:", "stack:")
	// No duplication of the structured header on separate adjacent lines.
	notContains(t, out, "code=defect code=defect")
}
