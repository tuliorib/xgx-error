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
	// merges inlined frames via CallersFrames; callers rarely need the bit.
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
// skip rules (per runtime.Callers):
//
//	0 -> this function (captureStackDefault)
//	1 -> its caller (captureStack)
//	2 -> caller's caller (e.g., WithStack)
//	...
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
	// +2 accounts for runtime.Callers itself plus captureStack; skip is relative
	// to the caller of captureStackDefault/captureStack.
	pc := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pc)
	if n == 0 {
		return nil
	}
	pc = pc[:n]

	frames := runtime.CallersFrames(pc)
	var out Stack
	out = make(Stack, 0, n) // prealloc for readability; indexing variant offers negligible gain

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
