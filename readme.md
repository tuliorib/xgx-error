# xgx-error · tiny core for precise, composable errors (Go 1.25)

Minimal by design: a **small, sharp error model** with perfect stdlib interop, ergonomic context, selective stacks, and zero policy baked in. Build adapters (HTTP, slog, OTel, JSON) **on top**—not in the core.

* **Go**: 1.25+ (currently pre-release—see [Go 1.25 release notes][1]; uses new `testing/synctest` in tests)
* **Interop-first**: plays nicely with `errors.Is/As`, `errors.Join`, and formatting via `fmt.Formatter`. ([Go Packages][2])

---

## Why this exists

Most projects need: (1) a clear taxonomy, (2) rich context without surprises, (3) chains/joins that `errors.Is/As` traverse, (4) optional stacks, and (5) **no** logging/HTTP baggage in the core. xgx-error gives you that—and nothing more.

* `errors.Is/As` walk both **single** (`Unwrap() error`) **and multi** (`Unwrap() []error`) unwrap graphs; `errors.Join` returns a multi-unwrapper the stdlib traverses. We align to that model exactly. ([Go Packages][2])
* Stacks captured via `runtime.Callers` + `runtime.CallersFrames` (handles inlined frames correctly). ([Go Packages][3])
* Verbose formatting driven by `fmt.Formatter` with `%+v` for diagnostics; `%v` stays concise. ([Go Packages][4])
* Tests use the new **`testing/synctest`** package landing in Go 1.25 to deterministically validate copy-on-write immutability and concurrency. ([pkg.go.dev/testing/synctest][5])

---

## Install

```bash
go get github.com/tuliorib/xgx-error@latest
```

Requires Go **1.25+** (currently in preview; see [release notes][1]).

---

## Core ideas (no policy in core)

* **Taxonomy**:

  * `Failure` (expected domain/infra errors; e.g., `not_found`, `invalid`, `unavailable`)
  * `Defect`   (programmer bugs; always capture a stack)
  * `Interrupt` (cancellation/deadline; unwraps to `context.Canceled` or `DeadlineExceeded`)
* **Context**: immutable, append-only `[]Field` internally; `Context()` returns a **copy** `map[string]any`.
* **Stacks**: off by default for `Failure`, on for `Defect`, opt-in via `.WithStack()`/`.WithStackSkip()`.
* **Interop**: `Unwrap()` chains, and helpers thin over `errors.Join`. ([Go Packages][2])

---

## Quick start

```go
import (
  "errors"
  xerr "github.com/xgx-io/xgx-error"
)

func repoGetUser(id int) (User, error) {
  // domain miss
  return User{}, xerr.NotFound("user", id)
}

func serviceGetUser(id int) error {
  u, err := repoGetUser(id)
  if err != nil {
    // add message + context immutably
    return xerr.Wrap(err, "query failed", "table", "users", "user_id", id)
  }
  _ = u
  return nil
}

func handler() error {
  err := serviceGetUser(42)
  if err == nil { return nil }

  // stdlib checks
  if errors.Is(err, xerr.NotFound("user", 0)) { /* matches by type/code via As/Code */ }

  // Predicates
  if xerr.IsDefect(err) { /* page on-call */ }

  // Format: %v concise; %+v verbose with stack/cause/context
  // fmt.Printf("%+v\n", err)
  return err
}
```

### Parallel & sequential composition

```go
// Parallel validation: use stdlib join; xgx Flatten/Walk for programmatic traversal
err := errors.Join(
  xerr.Invalid("email", "bad format"),
  xerr.Invalid("age", "too young"),
)
if err != nil {
  // errors.Is/As see both branches; Flatten collects the leaves
  leaves := xerr.Flatten(err) // []error
  _ = leaves
}
```

`errors.Join` returns an error implementing **`Unwrap() []error`**; `errors.Is/As` perform pre-order, depth-first traversal across the graph. ([GitHub][6])

---

## API snapshot (tiny core)

**Constructors**

* Domain: `NotFound`, `Invalid`, `Unprocessable`, `BadRequest`, `Unauthorized`, `Forbidden`, `Conflict`, `TooManyRequests`
* Infra: `Internal`, `Timeout`, `Unavailable`
* Defect/Interrupt: `Defect`, `Interrupt`, `InterruptDeadline`

**Fluent (non-mutating)**

* `.Ctx(msg, kv...)` — appends message segments using `": "` as the separator and returns a fresh error with merged context.
* `.CtxLast(msg, kv...)` — replaces the current message while appending optional context (great for rewriting summary text).
* `.CtxBound(msg, limit, kv...)` — appends a segment but keeps only the most recent `limit` message parts (defensive against unbounded growth).
* `.With(key, val)` — attaches a single field immutably.
* `.Code(code)` — overrides the classification (no-op for defects/interrupts by design).
* `.WithStack()` / `.WithStackSkip(skip)` — opt-in stack capture with configurable frame skipping.

**Helpers**

* `From`, `Wrap`, `With`, `Recode`, `WithStack`, `WithStackSkip`
* `Flatten`, `Walk`, `Root`
* Predicates: `IsDefect`, `IsInterrupt`, `IsRetryable`, `HasCode`, `CodeOf`, `Has`

`Flatten` collects only leaf errors into a slice, whereas `Walk` visits every node in pre-order and can short-circuit when the callback returns `false`.

**Formatting**

* `%v` / `%s`: concise (`Error()`)
* `%+v`: verbose (code, msg, ordered context, cause (recursively `%+v`), stack if present) via `fmt.Formatter`. ([Go Packages][4])

**Stacks**

* Captured with `runtime.Callers` + `runtime.CallersFrames` for accurate symbolization. ([Go Packages][3])

---

## What’s intentionally **not** in core

* HTTP status mapping, logging, JSON, OTel, retries/backoff.
  Put these in **small add-on modules**:

  * `xgx-error-http` (code→status; `WriteError`, Problem Details per [RFC 9457][8])
  * `xgx-error-slog` (adapter for `log/slog`) ([Go.dev][7])
  * `xgx-error-json` (classic `encoding/json` + optional `encoding/json/v2`)
  * `xgx-error-otel` (trace/span helpers)

This keeps the model pure and reusable.

---

## Testing & determinism

The repo’s concurrency tests run inside a **`testing/synctest` bubble**: virtual time, deterministic scheduling, and `synctest.Wait()` to advance when all goroutines are blocked—perfect for proving copy-on-write immutability without flaky sleeps. This package lands with Go 1.25. ([pkg.go.dev/testing/synctest][5])

---

## Formatting & stacks: what to expect

* `%v` shows a short one-liner. `%+v` prints:

  ```
  code=<code> msg="..."
  ctx: key=value ...
  cause: <recursively %+v>
  stack:
    pkg.Func file.go:123
    ...
  ```

  This mirrors community practice for verbose diagnostics while staying stdlib-only via `fmt.Formatter`. ([Go Packages][4])

---

## Performance

| Operation | Allocations (approx.) | Notes |
| --- | --- | --- |
| Success path (no error) | 0 | Hot paths stay allocation-free. |
| Constructors (e.g., `NotFound`) | 1 | Allocate the concrete error; context slice is pre-sized for provided fields. |
| `.Ctx` / `.With` | 1 | Copy-on-write context slice; `.Ctx` appends message segments using `": "` as the separator. |
| `.CtxLast` / `.CtxBound` | 1 | Replace or bound message history while preserving copy-on-write context semantics. |
| Stack capture (`Defect`, `WithStack*`) | 1 buffer + ~64 frames | Captures up to 64 PCs via `runtime.Callers`; resolved lazily when formatting. |
| Traversal helpers (`Flatten`, `Walk`) | Leaves slice / none | `Flatten` allocates a slice sized to leaf count; `Walk` uses an internal stack only. |

Traversal helpers execute depth-first search with **O(nodes)** time and **O(depth)** memory. Stack capture work is performed only when you opt in (or when emitting `Defect`).

(Always benchmark in your environment.)

---

## Versioning & compatibility

* Target: **Go 1.25+**
* Uses only stdlib APIs; adheres to the behavior documented for `errors.Is/As`, `errors.Join`, `fmt`, `runtime.CallersFrames`, and `testing/synctest`. ([Go Packages][2])

---

## FAQ

**Why not embed slog / HTTP mapping in the core?**
To keep the model reusable across CLIs, services, and libraries, we keep **policy out of core**. If you need slog, add `xgx-error-slog` (structured logging arrived in stdlib in Go 1.21). ([Go.dev][7])

**Do you support joined errors?**
Yes—use `errors.Join`. Our helpers traverse `Unwrap() []error` graphs and the stdlib’s `errors.Is/As` already do this by design. ([Go Packages][2])

**How are stacks captured?**
Via `runtime.Callers` and resolved with `runtime.CallersFrames`, which handles inlined frames; no external deps. ([Go Packages][3])

---

## Contributing

Small surface area, high bar for correctness. Please include:

* Unit tests for `errors.Is/As` behavior (including joined errors)
* `%v/%+v` formatting assertions (don’t match full stacks; look for sections)
* Synctest-backed COW tests (Go 1.25) ([Go Tour][5])

---

## License

MIT (see `LICENSE`).

[1]: https://tip.golang.org/doc/go1.25?utm_source=chatgpt.com "Go 1.25 Release Notes (pre-release)"
[2]: https://pkg.go.dev/errors?utm_source=chatgpt.com "errors package"
[3]: https://pkg.go.dev/runtime?utm_source=chatgpt.com "runtime package"
[4]: https://pkg.go.dev/fmt?utm_source=chatgpt.com "fmt package - fmt"
[5]: https://pkg.go.dev/testing/synctest?utm_source=chatgpt.com "testing/synctest package"
[6]: https://github.com/golang/go/issues/69586?utm_source=chatgpt.com "add example around unwrapping errors.Join · Issue #69586"
[7]: https://go.dev/blog/slog?utm_source=chatgpt.com "Structured Logging with slog"
[8]: https://www.rfc-editor.org/rfc/rfc9457 "Problem Details for HTTP APIs (RFC 9457)"
