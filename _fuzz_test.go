package xgxerror

import (
	"strconv"
	"strings"
	"testing"
)

func FuzzCtxFromKV(f *testing.F) {
	f.Add("key value", byte(0), byte(0))
	f.Add("k1 v1 k2 v2", byte(1), byte(1))
	f.Add("odd onlykey", byte(0), byte(1))

	f.Fuzz(func(t *testing.T, raw string, toggle byte, odd byte) {
		tokens := strings.Fields(raw)
		var kv []any
		for i, token := range tokens {
			kv = append(kv, "key"+strconv.Itoa(i))
			kv = append(kv, token)
		}
		if len(kv) == 0 {
			kv = append(kv, "empty", raw)
		}
		if toggle%3 == 0 {
			kv[0] = int(toggle) // non-string key should be ignored
		}
		if odd%2 == 1 {
			kv = kv[:len(kv)-1]
		}

		fields := ctxFromKV(kv...)
		for _, field := range fields {
			if field.Key == "" {
				continue
			}
			if strings.HasPrefix(field.Key, "key") {
				idxStr := strings.TrimPrefix(field.Key, "key")
				if _, err := strconv.Atoi(idxStr); err != nil {
					t.Fatalf("unexpected key format: %q", field.Key)
				}
			}
		}
	})
}
