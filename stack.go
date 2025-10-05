// stack.go — selective stack capture for xgx-error core.
//
// Design goals:
//   - Interop & correctness: use runtime.Callers + runtime.CallersFrames for
//     accurate frame resolution (handles inlining correctly).
//   - Minimal policy: no global toggles here; callers opt in via WithStack*.
//   - Pragmatic performance: bounded depth, cheap defaults, no allocations on
//     success paths (only when capture is requested).
//
// References:
//   - runtime.Callers / CallersFrames docs and example
//   - Prefer CallersFrames over FuncForPC for inlined frames
//   - Callers skip semantics (0 = Callers, 1 = its caller)
//   - Go 1.25 context (no API breaking changes needed here)
package xgxerror

import (
	"runtime"
)

// Frame represents a single call site in a stack trace.
type Frame struct {
	PC       uintptr // program counter of the call return
	File     string  // absolute file path (as provided by runtime)
	Line     int     // line number
	Function string  // fully-qualified function name (pkg.Func or method)
	// Note: we intentionally omit "Inlined" because runtime.Frame already
	// expands inlined frames via CallersFrames; callers rarely need the bit.
}

// Stack is a slice of Frames from most recent call outward.
type Stack []Frame

const (
	// defaultMaxDepth is a conservative bound that captures meaningful
	// context without excessive work on exceptional paths.
	defaultMaxDepth = 64
)

// captureStackDefault captures a stack skipping 'skip' frames, with a
// conservative default depth bound.
//
// Skip model for a typical call chain:
//
//   WithStack → WithStackSkip → captureStackDefault → captureStack → runtime.Callers
//
// The skip parameter here is *additional* to the internal helpers. Internally
// we ensure user-visible stacks begin at (or very near) the user call site by
// adding +3 in captureStack (to skip runtime.Callers, captureStack, and
// captureStackDefault). Any extra 'skip' provided by callers is applied on top.
func captureStackDefault(skip int) Stack {
	return captureStack(skip, defaultMaxDepth)
}

// captureStack captures up to maxDepth frames, skipping 'skip' initial frames.
// It returns a resolved Stack with file, line, and function names.
//
// Notes:
//   - We allocate a small PC buffer sized by maxDepth and let Callers trim it.
//   - We always reslice to the number of PCs actually written.
//   - We resolve frames via CallersFrames to handle inlined calls correctly.
func captureStack(skip, maxDepth int) Stack {
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}

	// Skip accounting:
	//   • +1 for runtime.Callers itself
	//   • +1 for captureStack
	//   • +1 for captureStackDefault
	// Therefore we add +3 to place the first recorded frame at (or very near)
	// the user-visible call site (e.g., the caller of WithStack/WithStackSkip).
	pc := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+3, pc)
	if n == 0 {
		return nil
	}
	pc = pc[:n]

	frames := runtime.CallersFrames(pc)
	out := make(Stack, 0, n)

	for {
		fr, more := frames.Next()
		out = append(out, Frame{
			PC:       fr.PC,
			File:     fr.File,
			Line:     fr.Line,
			Function: fr.Function,
		})
		if !more {
			break
		}
	}
	return out
}
