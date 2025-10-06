# xerr-error

**Production-ready, policy-free error handling for Go.**

Tiny, fast, stdlib-aligned error core with:

* **Immutable fluent API** (copy-on-write semantics)
* **Typed context fields** (zero-alloc fast path for native errors)
* **Deliberate message control** (set-once, append, replace)
* **Selective stack capture** (opt-in at boundaries, automatic for defects)
* **First-class stdlib interop** (`errors.Is/As/Join`, `fmt.Formatter`)

---

## Install

```bash
go get github.com/tuliorib/xerr-error
```

---

## Go Version Support

* **Library:** Go **1.21+** (core uses only stable stdlib APIs)
* **Tests:** Some tests use Go **1.25** features and are guarded with `//go:build go1.25`

Recommended CI:
- `go1.21` — build + core tests
- `go1.25` — full test suite

---

## Quick Start

```go
package main

import (
    "fmt"
    xerr "github.com/tuliorib/xgx-error"
)

func main() {
    // Domain error with context
    err := xerr.NotFound("user", "u_42").
        Ctx("lookup in cache", "cache_key", "users:u_42")
    
    fmt.Printf("%v\n", err)
    // Output: not_found: user not found
    
    fmt.Printf("%+v\n", err)
    // Output: code=not_found msg="user not found"
    // ctx: entity=user id=u_42 cache_key=users:u_42
}
```

---

## Design Philosophy

**Three pragmatic categories:**
- **Failure** — expected domain outcomes (validation, not found, conflicts)
- **Defect** — programming errors/bugs (always capture stack)
- **Interrupt** — cooperative cancellation (unwraps to `context.Canceled/DeadlineExceeded`)

**Core principles:**
- **Policy-free**: No HTTP codes, retry logic, or logging in core
- **Immutable**: Every fluent method returns a new error
- **Interop-first**: Works seamlessly with `errors.Is/As/Join`
- **Minimal surface**: Small, stable API; extensions live in separate packages

---

## Message Semantics (Precise)

| Method | Message Behavior | Fields Behavior | Use Case |
|--------|-----------------|-----------------|----------|
| `Ctx(msg, kv...)` | **Set once** (only if empty) | **Always append** | Annotate with IDs; preserve original message |
| `MsgAppend(msg)` | Append with `": "` separator | No change | Build human-readable error trail |
| `MsgReplace(msg)` | Replace entirely | No change | Override when you know the right message |

**Example:**

```go
err := xerr.BadRequest("").              // msg = ""
    Ctx("invalid input", "field", "email"). // msg = "invalid input" (set once)
    Ctx("ignored", "user_id", 42).          // msg stays "invalid input", adds user_id
    MsgAppend("check format")               // msg = "invalid input: check format"

// Result:
// Message: "invalid input: check format"
// Context: {field: "email", user_id: 42}
```

---

## Constructors

### Domain / Validation (4xx-intent)

```go
xerr.BadRequest("malformed payload")
xerr.Unauthorized("missing token")
xerr.Forbidden("admin")                      // resource arg
xerr.NotFound("user", 42)                    // entity, id
xerr.Invalid("email", "format")              // field, reason
xerr.Unprocessable("age", "negative")        // field, reason
xerr.Conflict("duplicate key")
xerr.TooManyRequests("api")                  // resource arg
```

### Infrastructure (5xx-intent)

```go
xerr.Internal(dbErr)                         // wraps cause, captures stack
xerr.Timeout(250 * time.Millisecond)         // records duration
xerr.Unavailable("database")                 // service arg
```

### Programming Defects

```go
xerr.Defect(fmt.Errorf("nil pointer"))      // always captures stack
```

### Cooperative Interrupts

```go
xerr.Interrupt("user canceled")              // unwraps to context.Canceled
xerr.InterruptDeadline("deadline hit")       // unwraps to context.DeadlineExceeded
```

---

## Working with Any Error

Adapters that work with both xerr and foreign errors:

```go
// Pure conversion (nil-safe)
e := xerr.From(err)              // From(nil) == nil

// Wrap with context (creates error if nil)
e = xerr.Wrap(err, "fetching user", "user_id", 42)

// Add single field
e = xerr.With(err, "retry", 3)

// Change classification
e = xerr.Recode(err, xerr.CodeUnavailable)

// Capture stack at boundary
e = xerr.WithStack(err)          // opt-in stack capture
```

---

## Context & Typed Fields

Context is stored as an **append-only slice** internally (preserves order). Public reads return a **copy-on-read map** (safe for iteration).

**Semantics:**
- **Last-write-wins** per key
- **Empty keys are filtered** from Context() maps
- **Bounded context** via `CtxBound(msg, maxFields, kv...)` — keeps newest fields

### Typed Fields (Zero-Alloc Fast Path)

```go
// Define once, use everywhere
var (
    FUserID   = xerr.FieldOf[int64]("user_id")
    FTenantID = xerr.FieldOf[string]("tenant_id")
    FAttempt  = xerr.FieldOf[int]("attempt")
)

func doWork() error {
    err := xerr.Timeout(5 * time.Second)
    
    // Type-safe set
    err = FUserID.Set(err, 12345)
    err = FTenantID.Set(err, "acme")
    err = FAttempt.Set(err, 3)
    
    // Type-safe get (zero-alloc for native xerr errors)
    if id, ok := FUserID.Get(err); ok {
        fmt.Printf("User: %d\n", id)  // id is int64, no casting
    }
    
    // Or panic if missing (for tests/invariants)
    tenantID := FTenantID.MustGet(err)
    _ = tenantID
    
    return err
}
```

**Performance:** For native xerr errors, `Get` uses a zero-allocation lookup. Foreign errors fall back to `Context()` map (one allocation).

---

## Built-in Codes

**13 built-in codes** spanning common patterns:

```go
// Domain/Validation (8)
CodeBadRequest, CodeUnauthorized, CodeForbidden, CodeNotFound,
CodeConflict, CodeInvalid, CodeUnprocessable, CodeTooManyRequests

// Availability/Time (2)
CodeTimeout, CodeUnavailable

// Internal/Meta (3)
CodeInternal, CodeDefect, CodeInterrupt
```

**API:**

```go
// Get all built-ins (defensive copy)
for _, code := range xerr.BuiltinCodes() {
    fmt.Println(code)
}

// Check if code is built-in (O(1))
if xerr.CodeInternal.IsBuiltin() {
    // ...
}
```

> **Core is policy-free**: Codes have no HTTP mapping, retry semantics, or logging behavior. Build those in higher layers (e.g., `xerr-error-http`).

---

## Stack Capture

**When stacks are captured:**

| Constructor/Method | Stack Captured? | Rationale |
|-------------------|-----------------|-----------|
| `Internal(err)` | ✅ Always | Boundary; aids debugging |
| `Defect(err)` | ✅ Always | Programming bug |
| Domain constructors<br/>(NotFound, Invalid, etc.) | ❌ No | Cheap by design; opt-in via `.WithStack()` |
| `.WithStack()` / `.WithStackSkip(n)` | ✅ Opt-in | Precise capture site |
| Interrupt constructors | ❌ No | Cooperative cancel; no stack needed |

**Guidance:**
- Use `Internal(err)` at subsystem boundaries (always captures stack)
- Domain errors are lightweight; add `.WithStack()` only where debugging value exists
- Defects always include stacks (programming errors need maximum context)

---

## Formatting

| Format | Output |
|--------|--------|
| `%v`, `%s` | Concise, single-line: `code: message` |
| `%+v` | Verbose, multi-line with code, context, cause chain, stack frames |
| `%q` | Quoted concise format |

**Example:**

```go
err := xerr.Internal(dbErr).
    Ctx("query failed", "table", "users", "query_ms", 1250).
    WithStack()

fmt.Printf("%v\n", err)
// Output: internal: internal error

fmt.Printf("%+v\n", err)
// Output:
// code=internal msg="internal error"
// ctx: table=users query_ms=1250
// cause: <dbErr details>
// stack:
//   main.queryDB main.go:45
//   main.handler main.go:32
```

**For multi-error trees**, use `xerr.Join` instead of `errors.Join` to get recursive `%+v` formatting:

```go
err := xerr.Join(
    xerr.Invalid("email", "format"),
    xerr.Invalid("age", "negative"),
)
fmt.Printf("%+v\n", err)
// Each child renders with full structure
```

---

## Predicates & Traversal

```go
// Check code anywhere in error tree
xerr.HasCode(err, xerr.CodeNotFound)      // true if any node has code

// Get first code in DFS order
code := xerr.CodeOf(err)                 // "" if no codes found

// Classification predicates
xerr.IsDefect(err)                       // programming error?
xerr.IsInterrupt(err)                    // cancellation/timeout?
xerr.IsRetryable(err)                    // transient failure? (unavailable/timeout/rate-limit)

// Traversal
xerr.Walk(err, func(e error) bool {
    fmt.Printf("Node: %v\n", e)
    return true  // continue walking
})

leaves := xerr.Flatten(err)              // all leaf errors in DFS order
root := xerr.Root(err)                   // deepest cause (first leaf)

xerr.Has(err, target)                    // nil-safe errors.Is wrapper
```

---

## Interoperability

Works seamlessly with stdlib:

```go
// errors.Is/As traverse the entire graph
if errors.Is(err, context.Canceled) { /* ... */ }

var notFoundErr *xerr.NotFoundError
if errors.As(err, &notFoundErr) { /* ... */ }

// Interrupt unwraps to canonical context errors
interrupt := xerr.Interrupt("canceled")
errors.Is(interrupt, context.Canceled)  // true
```

**Cycle detection:** The unwrap/walk algorithms use a dual-guard strategy (comparable tokens + pointer identity) to handle graphs with non-comparable dynamic types safely. See `unwrap.go` for details.

---

## Usage Patterns

### HTTP Handler

```go
func toHTTPStatus(err error) int {
    switch {
    case xerr.HasCode(err, xerr.CodeNotFound):
        return 404
    case xerr.HasCode(err, xerr.CodeInvalid):
        return 400
    case xerr.HasCode(err, xerr.CodeUnauthorized):
        return 401
    case xerr.HasCode(err, xerr.CodeForbidden):
        return 403
    case xerr.HasCode(err, xerr.CodeConflict):
        return 409
    case xerr.HasCode(err, xerr.CodeTooManyRequests):
        return 429
    case xerr.IsInterrupt(err):
        return 499  // Client Closed Request
    default:
        return 500
    }
}
```

### Retry Logic

```go
func callWithRetry(ctx context.Context) error {
    for attempt := 1; attempt <= 3; attempt++ {
        err := doNetworkCall()
        if err == nil {
            return nil
        }
        
        if !xerr.IsRetryable(err) {
            return err  // permanent failure
        }
        
        backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
        select {
        case <-time.After(backoff):
            continue
        case <-ctx.Done():
            return xerr.InterruptDeadline("retry canceled")
        }
    }
    return xerr.Unavailable("service")
}
```

### Boundary Stack Capture

```go
func repositoryCall() error {
    return someDBError()  // foreign error, no stack
}

func serviceLayer() error {
    err := repositoryCall()
    if err != nil {
        // Capture stack once at boundary
        return xerr.Internal(err).
            Ctx("database operation failed", "service", "user-svc")
    }
    return nil
}
```

### Validation Aggregation

```go
func validateUser(u User) error {
    var errs []error
    
    if u.Email == "" {
        errs = append(errs, xerr.Invalid("email", "required"))
    }
    if u.Age < 0 {
        errs = append(errs, xerr.Invalid("age", "must be positive"))
    }
    
    if len(errs) > 0 {
        return xerr.Join(errs...)  // multi-error with %+v support
    }
    return nil
}
```

---

## Performance Notes

- **No-op paths avoid allocations** (e.g., `Ctx("", ...)` with no kv pairs)
- **Typed fields** use zero-alloc lookup for native xerr errors
- **Context slice** doesn't allocate until you actually add fields
- **Stack capture is explicit** (except `Internal` and `Defect`), keeping happy paths fast
- **Formatting** `%v` is cheap; `%+v` is lazy and only computed when rendered

**Benchmarking:** Add `bench_test.go` to your project comparing:
- xerr create/wrap vs `fmt.Errorf`
- Typed get vs map access
- `%v` vs `%+v` format cost

---

## FAQ

**Q: Why no `Get(key) (any, bool)` method?**  
**A:** Use typed fields (`FieldOf[T]`) for zero-alloc, type-safe reads. An untyped `Get` loses type safety and only helps foreign errors (which already fall back to the map).

**Q: Why drop non-string keys in `Ctx(kv...)`?**  
**A:** Prevents alignment bugs when keys/values are mixed. Dropping the entire pair is safer than guessing intent. If you need strict validation, check at call sites.

**Q: Can I define custom codes?**  
**A:** Yes! Codes are just `type Code string`. Define your own: `const CodeCustom xerr.Code = "custom_app_error"`. No central registry required.

**Q: Where are HTTP status codes / retry backoff / logging?**  
**A:** Out of scope by design. Build those in higher layers (e.g., adapters) that interpret xerr codes. Keep core stable and reusable.

**Q: Why three categories (Failure/Defect/Interrupt)?**  
**A:** Pragmatic classification that maps to real operational needs: expected outcomes, bugs, and cancellation. Keeps the mental model small.

**Q: How do I migrate from pkg/errors or cockroachdb/errors?**  
**A:** 
1. Replace `errors.Wrap` → `xerr.Wrap` or `xerr.Internal`
2. Replace `errors.New` → `xerr.New` or semantic constructors
3. Stack capture is opt-in; use `WithStack()` where needed
4. Context extraction changes from custom APIs to `Context()` or typed fields

---

## Contributing

**Principles:**
- Keep core minimal and policy-free
- Favor tests & documentation over API surface expansion
- Breaking changes require major version bump
- Read `unwrap.go` (cycle detection) and `stack.go` (frame capture) before modifying traversal/stack logic

**Pull requests welcome** for:
- Bug fixes
- Documentation improvements
- Test coverage expansion
- Performance optimizations (with benchmarks)

---

## License

MIT — see [LICENSE](LICENSE)

---

**xerr-error** — sharp, small, and exactly where you need it.