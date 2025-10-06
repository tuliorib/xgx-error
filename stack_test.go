// stack_test.go — verification of stack capture semantics and metadata.
package xgxerror

import (
	"strings"
	"testing"
)

// --- Helpers to build a known call chain -------------------------------------

// stackGrab calls captureStackDefault with the provided skipExtra and returns the stack.
func stackGrab(skipExtra int) Stack {
	return captureStackDefault(skipExtra+1)
}

func stackTestLevel2(skipExtra int) Stack {
	// First recorded frame with skipExtra=0 should be this function.
	return stackGrab(skipExtra)
}

func stackTestLevel1(skipExtra int) Stack {
	// With skipExtra=1, first recorded frame should be THIS function (caller of level2).
	return stackTestLevel2(skipExtra)
}

// --- Tests -------------------------------------------------------------------

func TestCaptureStack_UsesDefaultWhenMaxDepthZero(t *testing.T) {
	t.Parallel()

	s := captureStack(0, 0) // maxDepth<=0 → defaultMaxDepth
	if len(s) == 0 {
		t.Fatalf("expected non-empty stack when maxDepth=0 (default), got 0")
	}
	if len(s) > defaultMaxDepth {
		t.Fatalf("stack length exceeds defaultMaxDepth: len=%d default=%d", len(s), defaultMaxDepth)
	}
}

func TestCaptureStack_RespectsMaxDepthLimit(t *testing.T) {
	t.Parallel()

	const limit = 3
	s := captureStack(0, limit)
	if len(s) == 0 {
		t.Fatalf("expected some frames with small limit; got 0")
	}
	if len(s) > limit {
		t.Fatalf("expected <= %d frames; got %d", limit, len(s))
	}
}

func TestCaptureStackDefault_UsesDefaultDepth(t *testing.T) {
	t.Parallel()

	s := captureStackDefault(0)
	if len(s) == 0 {
		t.Fatalf("expected non-empty stack from captureStackDefault")
	}
	if len(s) > defaultMaxDepth {
		t.Fatalf("stack length exceeds defaultMaxDepth: len=%d default=%d", len(s), defaultMaxDepth)
	}
}

func TestCaptureStack_SkipExtraSkipsCorrectFrames(t *testing.T) {
	t.Parallel()

	// skipExtra = 0 → first frame should be stackTestLevel2
	s0 := stackTestLevel1(0)
	if len(s0) == 0 {
		t.Fatalf("got empty stack for skipExtra=0")
	}
	if !strings.HasSuffix(s0[0].Function, "stackTestLevel2") {
		t.Fatalf("expected first frame to be stackTestLevel2; got %q", s0[0].Function)
	}

	// skipExtra = 1 → first frame should be stackTestLevel1
	s1 := stackTestLevel1(1)
	if len(s1) == 0 {
		t.Fatalf("got empty stack for skipExtra=1")
	}
	if !strings.HasSuffix(s1[0].Function, "stackTestLevel1") {
		t.Fatalf("expected first frame to be stackTestLevel1; got %q", s1[0].Function)
	}
}

func TestCaptureStack_ReturnsNilWhenNoFramesCaptured(t *testing.T) {
	t.Parallel()

	// Use a very large skipExtra to skip beyond available frames so runtime.Callers returns 0.
	// This should cause captureStack(...) to return nil.
	const absurdSkip = 1 << 20
	s := captureStack(absurdSkip, 16)
	if s != nil {
		t.Fatalf("expected nil stack when overly large skip filters out all frames; got len=%d", len(s))
	}
}

func TestStack_MetadataPresence(t *testing.T) {
	t.Parallel()

	s := stackTestLevel1(0)
	if len(s) == 0 {
		t.Fatalf("empty stack")
	}

	// Check a handful of frames (at least the first few) for non-zero / non-empty fields.
	maxCheck := len(s)
	if maxCheck > 5 {
		maxCheck = 5
	}
	for i := 0; i < maxCheck; i++ {
		fr := s[i]
		if fr.PC == 0 {
			t.Fatalf("frame %d has zero PC", i)
		}
		if fr.Function == "" {
			t.Fatalf("frame %d has empty Function", i)
		}
		if fr.File == "" {
			t.Fatalf("frame %d has empty File", i)
		}
		if fr.Line <= 0 {
			t.Fatalf("frame %d has non-positive Line: %d", i, fr.Line)
		}
	}
}

func TestBaseSkip_HidesInternalHelpers(t *testing.T) {
	t.Parallel()

	// captureStackDefault should hide runtime.Callers, captureStack, and captureStackDefault.
	s := stackTestLevel1(0)
	if len(s) == 0 {
		t.Fatalf("empty stack")
	}
	first := s[0].Function

	if strings.Contains(first, "captureStack") || strings.Contains(first, "captureStackDefault") {
		t.Fatalf("internal helpers should not be the first recorded frame; got %q", first)
	}
}

func TestCapturedStack_StartsAtExpectedUserFrame(t *testing.T) {
	t.Parallel()

	// With skipExtra=0, first frame must be stackTestLevel2, i.e., the direct caller of captureStackDefault.
	s := stackTestLevel1(0)
	if !strings.HasSuffix(s[0].Function, "stackTestLevel2") {
		t.Fatalf("expected first user frame to be stackTestLevel2; got %q", s[0].Function)
	}
}

func TestPCValuesNonZero_FilePathsNonEmpty(t *testing.T) {
	t.Parallel()

	s := captureStackDefault(0)
	if len(s) == 0 {
		t.Fatalf("empty stack")
	}
	for i := range s {
		if s[i].PC == 0 {
			t.Fatalf("PC is zero at frame %d", i)
		}
		if s[i].File == "" {
			t.Fatalf("File is empty at frame %d", i)
		}
	}
}
