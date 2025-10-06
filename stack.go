// stack.go — selective stack capture for xgx-error core.
//
// Design goals:
//   - Interop & correctness: use runtime.Callers + runtime.CallersFrames for
//     accurate frame resolution (handles inlining correctly).
//   - Minimal policy: no global toggles here; callers opt in via WithStack*.
//   - Pragmatic performance: bounded depth, cheap defaults, allocate only when
//     capture is requested.
//
// Skip model (centralized):
//   - captureStack accounts for its own internal frames:
//       +1 for runtime.Callers
//       +1 for captureStack
//     => baseSkip = 2
//   - Because we commonly call captureStack via captureStackDefault, we set
//     baseSkip = 3 to also hide captureStackDefault by default.
//   - Callers pass ONLY their extra frames to skip (skipExtra).
//
// Typical chains:
//
//   WithStack → WithStackSkip → captureStackDefault → captureStack → runtime.Callers
//     • WithStackSkip(0) calls captureStackDefault(1) to skip itself.
//     • baseSkip (3) ensures we also hide captureStackDefault.
//
//   Defect(...) → captureStackDefault(0) → captureStack → runtime.Callers
//     • baseSkip (3) hides runtime.Callers, captureStack, captureStackDefault.
//
// Notes:
//   - We keep depth modest (defaultMaxDepth) and resolve frames via CallersFrames.
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
}

// Stack is a slice of Frames from most recent call outward.
type Stack []Frame

const (
	// defaultMaxDepth captures meaningful context without excessive work
	// on exceptional paths.
	defaultMaxDepth = 64
)

// captureStack captures a stack. The function accounts for its own internal frames:
// +1 for runtime.Callers, +1 for captureStack, and +1 for captureStackDefault.
// Callers pass only their extra skip (skipExtra).
func captureStack(skipExtra, maxDepth int) Stack {
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	pc := make([]uintptr, maxDepth)

	// See header notes: hide runtime.Callers, captureStack, captureStackDefault.
	const baseSkip = 3
	n := runtime.Callers(baseSkip+skipExtra, pc)
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

// captureStackDefault captures a stack with a conservative default depth,
// skipping only the additional frames requested by the caller (skipExtra).
func captureStackDefault(skipExtra int) Stack {
	return captureStack(skipExtra, defaultMaxDepth)
}
