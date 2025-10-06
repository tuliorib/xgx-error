// doc.go — package documentation for xgx-error
//
// Package xgxerror provides a tiny, policy-free error core with three concrete
// categories (Failure, Defect, Interrupt), an immutable context model, and
// fmt-based structured formatting. It is designed to be:
//   - Ergonomic at call sites (small surface, clear semantics)
//   - Interoperable with the stdlib (errors.Is/As/Join, fmt.Formatter)
//   - Policy-free (no HTTP/logging/retry rules in core)
//
// # Message Semantics
//
// xgxerror separates **message** operations from **context** (structured fields).
// The API is intentionally small and explicit:
//
//   - Ctx(msg, kv...):
//     Set-once message (only if empty) AND always add fields (no concatenation).
//     Use for boundary notes + structured context.
//   - MsgAppend(msg):
//     Append textual detail to the existing message using `": "` separator.
//   - MsgReplace(msg):
//     Overwrite the message entirely.
//
// Typical patterns:
//
//	err := NotFound("user", 42).
//	           Ctx("lookup failed", "tenant", "acme").
//	           MsgAppend("db timeout").
//	           With("attempt", 2)
//
// Results in a concise Error() and a rich %+v format (see formatting below).
//
// # When Are Stacks Captured?
//
// Stacks are captured deliberately to mark boundaries and avoid accidental cost.
// Use `.WithStack()` to opt in where needed.
//
//	+-------------------------------+-------------------+-------------------------------+
//	| Constructor / Operation       | Captures stack?   | Rationale                     |
//	+-------------------------------+-------------------+-------------------------------+
//	| Internal(err)                 | YES (always)      | Boundary; aids debugging       |
//	| Defect(err)                   | YES (always)      | Programming bug                |
//	| Timeout/Unavailable/...       | NO (default)      | Cheap classification by design |
//	| WithStack()/WithStackSkip(n)  | YES (opt-in)      | Precise capture site           |
//	| Interrupt/InterruptDeadline   | NO                 | Cooperative cancel; unwraps    |
//	+-------------------------------+-------------------+-------------------------------+
//
// Guidance:
//   - Use `Internal(err)` at boundaries; it always captures a stack (even if err is nil).
//   - Domain constructors (e.g., `Timeout`) are cheap; add `.WithStack()` only where useful.
//
// # Bounding & Order (Context)
//
// Context is an append-only `[]Field` with deterministic order. When you must
// cap growth (e.g., in retry loops), use `CtxBound(msg, max, kv...)`.
//
//   - Behavior: keeps the NEWEST fields, drops the oldest.
//   - Example: given [a, b, c, d, e] and max=3 → keeps [c, d, e] (newest).
//
// Guidance:
//   - For **must-keep IDs** (request_id, tenant), prefer **typed fields** and set them
//     early; bounded context will still keep the most recent assignment.
//   - Duplicate keys are allowed; “last write wins” when exposed via `Context()`.
//
// # Foreign Error Caveat
//
// Helpers like `HasCode`, `CodeOf`, and typed-field fast paths operate on errors
// that implement the xgxerror interfaces. “Foreign” errors (from other packages)
// with ad-hoc metadata **won’t** be interpreted unless they:
//   - Implement xgxerror’s internal interfaces, or
//   - Are wrapped by xgxerror constructors (e.g., `Internal(err)`).
//
// You can still attach structured context around foreign errors using `Ctx(...)`.
//
// # Formatting
//
// xgxerror implements `fmt.Formatter` for rich diagnostics:
//   - `%v`, `%s`   → concise, single-line `Error()`
//   - `%+v`        → verbose, multi-line (code, msg, ctx, cause, stack)
//   - `%q`         → quoted `Error()`
//
// Joining multiple errors: use `xgxerror.Join` for `%+v`-aware recursion.
// `errors.Is/As` traverse via `Unwrap()` (including multi-error unwraps).
//
// # Performance Notes
//
// The core is designed for low overhead in the common path while remaining
// precise when you need detail.
//
//   - **Copy-on-write**: all fluent methods return new values (immutability).
//   - No-op paths avoid allocations (e.g., Ctx with no kv keeps existing slice).
//   - `ctxCloneAppend` allocates only when appending new fields.
//   - **Typed fields**: zero-alloc fast path for native xgxerror values; on foreign
//     errors, `TypedField.Get` falls back to `Context()` which builds a map (alloc).
//   - **Stack capture**: costs only when you call `Internal/Defect` (always) or
//     opt in with `WithStack()`.
//   - **Formatting**: verbose `%+v` is lazy; concise `%v` remains cheap.
//
// # Interop
//
//   - `errors.Is/As/Join` work as expected; unwrap chains are respected.
//   - Interrupt errors unwrap to canonical `context` sentinels
//     (`context.Canceled`, `context.DeadlineExceeded`).
//   - The public `Context()` returns a copy-on-read `map[string]any` with last-write-wins.
//
// # Minimal Surface, Clear Semantics
//
// The v1 surface is intentionally small to remain ergonomic:
//   - Ctx / CtxBound
//   - MsgAppend / MsgReplace
//   - With / WithStack / WithStackSkip
//   - Domain & infra constructors (NotFound, Invalid, Timeout, Internal, …)
//
// See examples in examples_test.go for runnable demonstrations.
package xgxerror
