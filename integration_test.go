//go:build go1.20

// _integration_test.go — cross-cutting integration tests for xgx-error.
package xgxerror

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIntegration_DeepMixedChain_IsAs(t *testing.T) {
	t.Parallel()

	// failure → defect (cause=failure), and in parallel branch an interrupt.
	leaf := NotFound("user", 42)
	def := Defect(leaf)              // captures stack; unwraps to leaf
	intt := Interrupt("stopping")    // unwraps to context.Canceled
	top := errors.Join(def, intt)    // multi-branch tree

	// stdlib traversal should find both branches/leaves
	if !errors.Is(top, context.Canceled) {
		t.Fatalf("errors.Is(top, context.Canceled)=false")
	}
	if !errors.Is(top, leaf) {
		t.Fatalf("errors.Is(top, leaf)=false (should find NotFound leaf through defect branch)")
	}

	// Our predicates should see codes in the graph.
	if !HasCode(top, CodeNotFound) {
		t.Fatalf("HasCode(top, not_found)=false")
	}
	if CodeOf(def) != CodeDefect {
		t.Fatalf("CodeOf(defect) != defect")
	}
}

func TestIntegration_LargeJoin_FlattensCorrectly(t *testing.T) {
	t.Parallel()

	var parts []error
	want := 0
	for i := 0; i < 12; i++ {
		// Mix stdlib and xgx leaves; include some nils that Join should discard.
		if i%3 == 0 {
			parts = append(parts, nil) // ignored
		} else if i%2 == 0 {
			parts = append(parts, Invalid(fmt.Sprintf("f%d", i), "bad"))
			want++
		} else {
			parts = append(parts, errors.New(fmt.Sprintf("e%d", i)))
			want++
		}
	}
	j := errors.Join(parts...)
	leaves := Flatten(j)
	if len(leaves) != want {
		t.Fatalf("Flatten(join) len=%d, want=%d; leaves=%v", len(leaves), want, leaves)
	}
	// sanity: every leaf is reachable via errors.Is from the join
	for _, l := range leaves {
		if !errors.Is(j, l) {
			t.Fatalf("errors.Is(join, leaf)=false for %v", l)
		}
	}
}

func TestIntegration_FluentChaining_PreservesState(t *testing.T) {
	t.Parallel()

	e := NotFound("file", "/tmp/a").
		Ctx("ignored-by-v1-rule", "hint", "case-sensitive").
		With("txn", "abc123").
		Code(CodeConflict).
		WithStack()

	// Message should remain from NotFound constructor; v1 .Ctx(...) does NOT concatenate.
	if !strings.Contains(fmt.Sprintf("%v", e), "file not found") {
		t.Fatalf("message was not preserved from constructor")
	}
	if CodeOf(e) != CodeConflict {
		t.Fatalf("CodeOf after Code() != conflict")
	}
	m := e.Context()
	if m["hint"] != "case-sensitive" || m["txn"] != "abc123" {
		t.Fatalf("context missing fields; got %v", m)
	}
	// Stack presence: %+v should include a "stack:" section.
	if !strings.Contains(fmt.Sprintf("%+v", e), "\nstack:") {
		t.Fatalf("expected stack section in verbose formatting")
	}
}

func TestIntegration_Concurrent_CopyOnWrite_Safety(t *testing.T) {
	t.Parallel()

	base := BadRequest("oops") // immutable base

	var wg sync.WaitGroup
	const N = 64
	results := make([]Error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			results[i] = base.With(fmt.Sprintf("k%d", i), i)
		}()
	}
	wg.Wait()

	// Base must remain unchanged.
	if len(base.Context()) != 0 {
		t.Fatalf("base mutated; ctx=%v", base.Context())
	}
	// Derived errors must carry their own fields.
	for i := 0; i < N; i++ {
		if results[i].Context()[fmt.Sprintf("k%d", i)] != i {
			t.Fatalf("derived #%d missing its own field", i)
		}
	}
}

func TestIntegration_CtxBound_DoesNotGrowUnbounded(t *testing.T) {
	t.Parallel()

	e := BadRequest("x")
	for i := 0; i < 1000; i++ {
		e = e.CtxBound("", 10, fmt.Sprintf("k%d", i), i)
	}
	if len(e.Context()) != 10 {
		t.Fatalf("CtxBound did not enforce bound; got %d fields", len(e.Context()))
	}
}

func TestIntegration_StackCapturedOnce_AcrossBoundary(t *testing.T) {
	t.Parallel()

	// Internal(err) captures a stack once; subsequent Wrap() must not add a new stack section.
	cause := errors.New("db timeout")
	e1 := Internal(cause) // boundary capture
	v1 := fmt.Sprintf("%+v", e1)
	if !strings.Contains(v1, "\nstack:") {
		t.Fatalf("Internal should capture stack")
	}

	e2 := Wrap(e1, "while fetching", "service", "users")
	v2 := fmt.Sprintf("%+v", e2)
	// Still exactly one "stack:" section in formatted output.
	if strings.Count(v2, "\nstack:") != 1 {
		t.Fatalf("expected single stack section after Wrap; got:\n%s", v2)
	}
}

func TestIntegration_Formatting_DeepCauses(t *testing.T) {
	t.Parallel()

	leaf := errors.New("root-cause")
	mid := Wrap(leaf, "mid", "k", "v")
	top := Internal(mid).Ctx("top", "id", 7).WithStack()

	out := fmt.Sprintf("%+v", top)
	for _, want := range []string{
		"code=internal", `msg="internal error"`,
		"ctx:", "id=", "7",
		"cause:",
		// recursive cause details
		"code=internal", // from Wrap(leaf,...) default code for non-xgx was internal
		`msg="mid"`, "k=", "v",
		"root-cause",
		"stack:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("verbose format missing %q\n---\n%s", want, out)
		}
	}
}

func TestIntegration_CustomCode_HasCode_CodeOf(t *testing.T) {
	t.Parallel()

	custom := Code("custom_app_code")
	e := Recode(BadRequest("x"), custom)

	if !HasCode(e, custom) {
		t.Fatalf("HasCode(custom) = false")
	}
	if CodeOf(e) != custom {
		t.Fatalf("CodeOf != custom")
	}
}

func TestIntegration_NilWrappingThroughoutChain(t *testing.T) {
	t.Parallel()

	var e Error
	e = Wrap(e, "first")               // creates internal
	e = With(e, "k", 1)               // adds field
	e = Recode(e, CodeTimeout)        // change code
	e = Ctx(e, "ignored-second")      // v1 rule: msg stays stable
	if CodeOf(e) != CodeTimeout {
		t.Fatalf("expected timeout code after Recode")
	}
	m := e.Context()
	if m["k"] != 1 {
		t.Fatalf("context propagation failed; got %v", m)
	}
}

func TestIntegration_RoundTrip_Create_Wrap_Join_Flatten_Format(t *testing.T) {
	t.Parallel()

	// Build three leaves: two xgx errors + one plain stdlib error.
	e1 := NotFound("doc", "a1")              // code=not_found
	e2 := Invalid("field", "blank")          // code=invalid
	e3 := errors.New("plain")                // no code

	// IMPORTANT: use xgxerror.Join (NOT errors.Join) so %+v recurses into children.
	joined := Join(e1, e2, e3)

	// Flatten should return all three leaves (order not guaranteed).
	leaves := Flatten(joined)
	if len(leaves) != 3 {
		t.Fatalf("Flatten(join) len=%d, want=3; leaves=%v", len(leaves), leaves)
	}
	// Sanity: each leaf reachable via errors.Is.
	for _, l := range leaves {
		if !errors.Is(joined, l) {
			t.Fatalf("errors.Is(joined, leaf)=false for %v", l)
		}
	}

	// %+v on xgxerror.Join must show structured sections for xgx leaves
	// (code/msg/etc.) and plain Error() for the stdlib leaf.
	out := fmt.Sprintf("%+v", joined)

	// Expect structured details for both xgx errors…
	wantSubs := []string{
		"code=not_found", `msg="doc not found"`,
		"code=invalid", `msg="invalid field"`,
		// …and the plain stdlib error line:
		"plain",
	}
	for _, s := range wantSubs {
		if !strings.Contains(out, s) {
			t.Fatalf("round-trip format missing %q\n---\n%s", s, out)
		}
	}
}


/*************** Real-world pattern sketches ****************/

func TestIntegration_HTTPHandler_NotFound_Internal_Format(t *testing.T) {
	t.Parallel()

	// Simulate handler path where 404 becomes internal at infra boundary.
	appErr := NotFound("user", 1).Ctx("", "route", "/users/1")
	boundary := Internal(appErr) // capture stack

	log := fmt.Sprintf("%+v", boundary)
	if !strings.Contains(log, "code=internal") || !strings.Contains(log, "cause:") {
		t.Fatalf("internal boundary log missing sections:\n%s", log)
	}
}

func TestIntegration_RetryLoop_IsRetryable(t *testing.T) {
	t.Parallel()

	err := errors.Join(Conflict("dup"), Timeout(250*time.Millisecond))
	if !IsRetryable(err) {
		t.Fatalf("Timeout branch should make IsRetryable true")
	}
}

func TestIntegration_Validation_MultiField_Join_Flatten(t *testing.T) {
	t.Parallel()

	v := errors.Join(
		Invalid("email", "format"),
		Invalid("age", "negative"),
	)
	leaves := Flatten(v)
	if len(leaves) != 2 {
		t.Fatalf("Flatten(validation) len=%d, want 2; %v", len(leaves), leaves)
	}
	if !HasCode(v, CodeInvalid) {
		t.Fatalf("HasCode(join, invalid)=false")
	}
}

func TestIntegration_RepositoryBoundary_From_WithStack_Recode(t *testing.T) {
	t.Parallel()

	sqlErr := errors.New("sql: connection refused")
	atRepo := From(sqlErr)      // convert to xgx
	atRepo = WithStack(atRepo)  // capture once
	atRepo = Recode(atRepo, CodeUnavailable)

	if CodeOf(atRepo) != CodeUnavailable {
		t.Fatalf("expected unavailable at repo boundary")
	}
	if !strings.Contains(fmt.Sprintf("%+v", atRepo), "\nstack:") {
		t.Fatalf("expected stack at repo boundary")
	}
}

func TestIntegration_PanicRecovery_DefectCapturesStack(t *testing.T) {
	t.Parallel()

	var err Error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = Defect(fmt.Errorf("%v", r))
			}
		}()
		panic("boom")
	}()

	if err == nil {
		t.Fatalf("expected recovered defect error")
	}
	if !strings.Contains(fmt.Sprintf("%+v", err), "\nstack:") {
		t.Fatalf("defect verbose format missing stack")
	}
}

func TestIntegration_ContextCancellation_InterruptDeadline(t *testing.T) {
	t.Parallel()

	e := InterruptDeadline("deadline hit")
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("errors.Is(e, DeadlineExceeded)=false")
	}
	// Ensure verbose format includes cause and no stack.
	out := fmt.Sprintf("%+v", e)
	if !strings.Contains(out, "cause:") || strings.Contains(out, "\nstack:") {
		t.Fatalf("interrupt %+v section expectations failed\n%s", e, out)
	}
}
