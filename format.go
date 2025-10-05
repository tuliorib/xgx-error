// format.go — fmt.Formatter implementations for xgx-error core.
//
// Behavior:
//
//   %s, %v   → concise string (Error()).
//   %+v      → verbose, structured multi-line format:
//                code=<code> msg="<message>"
//                ctx: key1=val1 key2=val2 ...
//                cause: <recursively formatted with %+v>
//                stack:
//                  funcA file.go:123
//                  funcB other.go:45
//
// Rationale:
//   - Keep core free of logging/HTTP/JSON policy; only fmt formatting.
//   - Deterministic context order via []Field from context.go.
//   - Defer cause formatting to fmt with %+v to preserve nested details.
package xgxerror

import (
	"fmt"
	"io"
)

// formatConcise writes the one-line message (delegates to Error()).
func formatConcise(w io.Writer, e error) {
	// ignore write errors in formatting paths
	_, _ = io.WriteString(w, e.Error())
}

// formatVerbose writes a structured multi-line representation.
// If stk is nil/empty, the stack section is omitted.
// If cause is non-nil, it is formatted with %+v to recurse verbosely.
func formatVerbose(w io.Writer, code Code, msg string, ctx fields, cause error, stk Stack) {
	// Header: code + msg
	if code != "" {
		_, _ = fmt.Fprintf(w, "code=%s ", code)
	}
	// Always quote message for clarity (even if empty).
	_, _ = fmt.Fprintf(w, "msg=%q", msg)

	// Context (ordered, space-separated key=val)
	if len(ctx) > 0 {
		_, _ = io.WriteString(w, "\nctx:")
		for _, f := range ctx {
			// Print key only if non-empty; values are %v for generality.
			if f.Key != "" {
				_, _ = fmt.Fprintf(w, " %s=%v", f.Key, f.Val)
			}
		}
	}

	// Cause
	if cause != nil {
		_, _ = io.WriteString(w, "\ncause: ")
		// Recurse with %+v so nested stacks/contexts render if available.
		_, _ = fmt.Fprintf(w, "%+v", cause)
	}

	// Stack frames (most recent first)
	if len(stk) > 0 {
		_, _ = io.WriteString(w, "\nstack:")
		for _, fr := range stk {
			// Function names are fully-qualified (pkg.Func / recv.method).
			// File paths come from runtime; we print as-is for accuracy.
			_, _ = fmt.Fprintf(w, "\n  %s %s:%d", fr.Function, fr.File, fr.Line)
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
		_, _ = fmt.Fprintf(s, "%q", e.Error())
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
			// Verbose: print code once and avoid duplicating "defect:" in msg.
			formatVerbose(s, CodeDefect, e.plainMsgOrCause(), e.ctx, e.cause, e.stk)
			return
		}
		// Concise: delegate to Error(), which includes "defect: ..."
		formatConcise(s, e)
	case 's':
		formatConcise(s, e)
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	default:
		formatConcise(s, e)
	}
}

// plainMsgOrCause returns the message/cause WITHOUT "defect: " prefix,
// to avoid duplication in verbose output where code=defect is already present.
func (e *defectErr) plainMsgOrCause() string {
	if e.msg != "" {
		return e.msg
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return ""
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
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	default:
		formatConcise(s, e)
	}
}
