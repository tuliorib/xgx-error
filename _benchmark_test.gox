package xgxerror

import (
	"errors"
	"testing"
)

func BenchmarkConstructors(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NotFound("user", i)
	}
}

func BenchmarkCtxAppend(b *testing.B) {
	base := NotFound("user", 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = base.Ctx("step", "idx", i)
	}
}

func BenchmarkCtxBound(b *testing.B) {
	base := NotFound("user", 1).Ctx("phase one").Ctx("phase two")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = base.CtxBound("phase", 3, "attempt", i)
	}
}

func BenchmarkWithStack(b *testing.B) {
	base := New("boom")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = base.WithStack()
	}
}

func BenchmarkDefect(b *testing.B) {
	cause := errors.New("boom")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Defect(cause)
	}
}

func buildDeepJoin(depth int) error {
	err := NotFound("leaf", depth)
	for i := depth - 1; i >= 0; i-- {
		err = errors.Join(err, Invalid("field", "bad"))
		if i%2 == 0 {
			err = Wrap(err, "layer", "idx", i)
		}
	}
	return err
}

func BenchmarkFlattenDeep(b *testing.B) {
	err := buildDeepJoin(64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Flatten(err)
	}
}

func BenchmarkWalkDeep(b *testing.B) {
	err := buildDeepJoin(64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Walk(err, func(error) bool { return true })
	}
}
