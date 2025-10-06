package xgxerror

import (
	"errors"
	"testing"
	"testing/quick"
)

func TestQuickFlattenJoinContainsLeaves(t *testing.T) {
	property := func(msgA, msgB string) bool {
		a := New(msgA)
		b := New(msgB)
		joined := errors.Join(a, b)
		leaves := Flatten(joined)
		foundA := false
		foundB := false
		for _, leaf := range leaves {
			if errors.Is(leaf, a) {
				foundA = true
			}
			if errors.Is(leaf, b) {
				foundB = true
			}
		}
		return foundA && foundB
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatalf("flatten(join) property failed: %v", err)
	}
}

func TestQuickErrorsIsReflexive(t *testing.T) {
	property := func(msg string) bool {
		err := New(msg)
		return errors.Is(err, err)
	}
	if err := quick.Check(property, nil); err != nil {
		t.Fatalf("errors.Is should be reflexive: %v", err)
	}
}
