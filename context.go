// context.go — minimal immutable context for xgx-error core.
//
// Ctx semantics (call-sites):
//   - Set-once message (only if empty) and always add fields.
//
// Design:
//   - Internal representation: append-only []Field (deterministic order).
//   - Builders are non-mutating: return NEW slices (no aliasing).
//   - Public view for callers: copy-on-read map[string]any.
//
// Rationale:
//   - Go map iteration order is unspecified; slice preserves insertion order.
//   - Slice append may re-use capacity; we avoid aliasing by allocating when
//     we actually append. No-op paths avoid extra allocations.
//
// Note: All identifiers here are unexported except Field; other files in the
// same package use these helpers to implement Error methods.
package xgxerror

// Field represents a single contextual key-value pair attached to an error.
// Keys SHOULD be snake_case for consistency; the core does not enforce policy.
type Field struct {
	Key string
	Val any
}

// fields is the internal immutable representation of context.
// Treat it as append-only; never modify elements in place once published.
type fields []Field

// emptyFields is a canonical empty context.
var emptyFields = make(fields, 0)

// ctxCloneAppend returns a slice with dst's contents followed by add.
//
// Rules:
//   - If add is empty:
//   - If dst is empty → return emptyFields.
//   - If dst is non-empty → return dst as-is (no allocation, no copy).
//     This is safe because callers MUST NOT mutate returned slices.
//   - If add is non-empty: allocate a fresh backing array to avoid aliasing.
func ctxCloneAppend(dst fields, add ...Field) fields {
	n := len(dst)
	m := len(add)
	if m == 0 {
		if n == 0 {
			return emptyFields
		}
		// v1.1 micro-alloc optimization: return dst directly for no-op appends.
		// Immutability is preserved by contract (callers never mutate).
		return dst
	}
	out := make(fields, n+m)
	copy(out, dst)
	copy(out[n:], add)
	return out
}

// ctxFromKV parses a variadic list of key-value arguments into fields.
//
// Rules (normative):
//   - Pairs are read left-to-right as (key, value).
//   - Keys MUST be strings; a non-string “key” causes the ENTIRE PAIR to be
//     dropped (the key and its following value, if any). This avoids surprising
//     misalignment where a value becomes the next pair’s key.
//   - A trailing key with no value becomes (key, nil).
//
// Example (why we drop the whole pair):
//
//	// INPUT (bad first key):
//	ctxFromKV(123, "v1", "k2", "v2")
//	// NEW behavior (pair dropped, keeps alignment):
//	//   → [{Key:"k2", Val:"v2"}]
func ctxFromKV(kv ...any) fields {
	if len(kv) == 0 {
		return emptyFields
	}
	out := make(fields, 0, len(kv)/2+1)
	for i := 0; i < len(kv); {
		k, ok := kv[i].(string)
		if !ok {
			// Drop the entire pair (key and its following value, if any)
			// to prevent misalignment of subsequent pairs.
			if i+1 < len(kv) {
				i += 2
			} else {
				i++
			}
			continue
		}
		var v any
		if i+1 < len(kv) {
			v = kv[i+1]
			i += 2
		} else {
			// Trailing key with no value → nil
			i++
		}
		out = append(out, Field{Key: k, Val: v})
	}
	if len(out) == 0 {
		return emptyFields
	}
	return out
}

// ctxToMap creates a NEW map from fields (copy-on-read).
// Semantics:
//   - Always returns a non-nil map (safe for mutation by the caller).
//   - Later duplicate keys overwrite earlier ones (last-write-wins).
//   - Empty keys are filtered out to avoid polluting caller maps.
func ctxToMap(fs fields) map[string]any {
	m := make(map[string]any, len(fs))
	for _, f := range fs {
		if f.Key == "" {
			continue // filter empty keys
		}
		m[f.Key] = f.Val
	}
	return m
}
