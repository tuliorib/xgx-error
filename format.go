// format.go — fmt.Formatter implementations for xgx-error core.
//
// Behavior:
//
//	%s, %v   → concise string (Error()).
//	%+v      → verbose, structured multi-line format:
//	             code=<code> msg="<message>"
//	             ctx: key1=val1 key2=val2 ...
//	             cause: <recursively formatted with %+v>
//	             stack:
//	               funcA file.go:123
//	               funcB other.go:45
//
// Rationale:
//   - Keep core free of logging/HTTP/JSON policy; only fmt formatting.
//   - Deterministic context order via []Field from context.go.
//   - Defer cause formatting to fmt with %+v to preserve nested details.
//
// References:
//   - fmt.Formatter contract & verbs. :contentReference[oaicite:1]{index=1}
package xgxerror

import (
	"fmt"
	"io"
)

// formatConcise writes the one-line message (delegates to Error()).
func formatConcise(w io.Writer, e error) {
	io.WriteString(w, e.Error())
}

// formatVerbose writes a structured multi-line representation.
// If stk is nil/empty, the stack section is omitted.
// If cause is non-nil, it is formatted with %+v to recurse verbosely.
func formatVerbose(w io.Writer, code Code, msg string, ctx fields, cause error, stk Stack) {
	// Header: code + msg
	if code != "" {
		fmt.Fprintf(w, "code=%s ", code)
	}
	// Always quote message for clarity (even if empty).
	fmt.Fprintf(w, "msg=%q", msg)

	// Context (ordered, space-separated key=val)
	if len(ctx) > 0 {
		io.WriteString(w, "\nctx:")
		for _, f := range ctx {
			// Print key only if non-empty; values are %v for generality.
			if f.Key != "" {
				fmt.Fprintf(w, " %s=%v", f.Key, f.Val)
			}
		}
	}

	// Cause
	if cause != nil {
		io.WriteString(w, "\ncause: ")
		// Recurse with %+v so nested stacks/contexts render if available.
		fmt.Fprintf(w, "%+v", cause)
	}

	// Stack frames (most recent first)
	if len(stk) > 0 {
		io.WriteString(w, "\nstack:")
		for _, fr := range stk {
			// Function names are fully-qualified (pkg.Func / recv.method).
			// File paths come from runtime; we print as-is for accuracy.
			fmt.Fprintf(w, "\n  %s %s:%d", fr.Function, fr.File, fr.Line)
		}
	}
}

// -----------------------------------------------------------------------------
// failureErr formatting
// -----------------------------------------------------------------------------

func (e *failureErr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			formatVerbose(s, e.code, e.msg, e.ctx, e.cause, e.stk)
			return
		}
		formatConcise(s, e)
	case 's':
		formatConcise(s, e)
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	default:
		formatConcise(s, e)
	}
}

// -----------------------------------------------------------------------------
// defectErr formatting (always has a captured stack at creation)
// -----------------------------------------------------------------------------

func (e *defectErr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// Defects are special: fixed code and always include stack.
			formatVerbose(s, CodeDefect, e.msgOrCause(), e.ctx, e.cause, e.stk)
			return
		}
		formatConcise(s, e)
	case 's':
		formatConcise(s, e)
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	default:
		formatConcise(s, e)
	}
}

func (e *defectErr) msgOrCause() string {
	if e.msg != "" {
		return "defect: " + e.msg
	}
	if e.cause != nil {
		return "defect: " + e.cause.Error()
	}
	return "defect"
}

// -----------------------------------------------------------------------------
// interruptErr formatting (no stack; unwraps to context errors)
// -----------------------------------------------------------------------------

func (e *interruptErr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// Interrupts print code + msg + ctx + cause (no stack).
			formatVerbose(s, CodeInterrupt, e.msg, e.ctx, e.cause, nil)
			return
		}
		formatConcise(s, e)
	case 's':
		formatConcise(s, e)
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	default:
		formatConcise(s, e)
	}
}
