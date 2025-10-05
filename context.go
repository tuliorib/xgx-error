// context.go — minimal immutable context for xgx-error core.
//
// Design:
//   • Internal representation: append-only []Field (deterministic order).
//   • Builders are non-mutating: return NEW slices (no aliasing).
//   • Public view for callers: copy-on-read map[string]any.
//
// Rationale:
//   • Go map iteration order is unspecified; slice preserves insertion order.
//   • Slice append may re-use capacity; we always allocate a fresh slice when
//     “mutating” to ensure copy-on-write semantics for safety.
//
// Note: All identifiers here are unexported except Field; other files in the
// same package use these helpers to implement Error methods.
package xgxerror

// Field represents a single contextual key-value pair attached to an error.
// Keys SHOULD be snake_case for consistency, but the core does not enforce it.
type Field struct {
	Key string
	Val any
}

// fields is the internal immutable representation of context.
// Treat it as append-only; never modify elements in place once published.
type fields []Field

// emptyFields is a canonical empty context.
var emptyFields = make(fields, 0)

// ctxCloneAppend returns a NEW slice with dst's contents followed by add.
// It always allocates a fresh backing array to avoid aliasing via append.
func ctxCloneAppend(dst fields, add ...Field) fields {
	n := len(dst)
	m := len(add)
	if m == 0 {
		if n == 0 {
			return emptyFields
		}
		// Return a deep copy to keep immutability guarantees for callers that
		// might retain references (rare, but cheap to ensure).
		out := make(fields, n)
		copy(out, dst)
		return out
	}
	out := make(fields, n+m)
	copy(out, dst)
	copy(out[n:], add)
	return out
}

// ctxFromKV parses a variadic list of key-value arguments into fields.
//
// Rules (normative):
//   • Pairs are read left-to-right as (key, value).
//   • Keys MUST be strings; a non-string “key” causes the ENTIRE PAIR to be
//     dropped (the key and its following value, if any). This avoids surprising
//     misalignment where a value becomes the next pair’s key.
//   • A trailing key with no value becomes (key, nil).
//
// Example (why we drop the whole pair):
//   // INPUT (bad first key):
//   ctxFromKV(123, "v1", "k2", "v2")
//   // OLD behavior (misaligned):
//   //   → [{Key:"v1", Val:"k2"}, {Key:"v2", Val:nil}]
//   // NEW behavior (pair dropped, keeps alignment):
//   //   → [{Key:"k2", Val:"v2"}]
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
	// Return a fresh slice with exact length (already exact via append).
	return out
}

// ctxToMap creates a NEW map from fields (copy-on-read).
// Later duplicate keys overwrite earlier ones (last-write-wins).
func ctxToMap(fs fields) map[string]any {
	if len(fs) == 0 {
		return nil
	}
	m := make(map[string]any, len(fs))
	for _, f := range fs {
		// Empty keys are allowed but discouraged; core does not enforce policy.
		m[f.Key] = f.Val
	}
	return m
}
