# Uber Go Style Guide (condensed)

> Source: distilled from the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
> (Apache 2.0, (c) Uber Technologies). Rules are paraphrased and shortened; examples are minimal.
> Read the original for full rationale. Where this file disagrees with the repo you are working in,
> follow the repo: Uber's own top rule is **be consistent**, and apply changes at package level.

## Contents
- [Types and interfaces](#types-and-interfaces)
- [Safety at API boundaries](#safety-at-api-boundaries)
- [Errors and panics](#errors-and-panics)
- [Concurrency](#concurrency)
- [Program structure](#program-structure)
- [Performance (hot paths only)](#performance-hot-paths-only)
- [Style and layout](#style-and-layout)
- [Initialization and declarations](#initialization-and-declarations)
- [Testing and options](#testing-and-options)
- [Tooling and where Uber is dated](#tooling-and-where-uber-is-dated)

## Types and interfaces

- **No pointer to an interface.** Pass interfaces by value; the data inside can still be a pointer. You only need a pointer-typed value when methods must mutate the underlying data.
- **Verify interface compliance at compile time** for exported types that must satisfy an interface and for families of types: `var _ http.Handler = (*Handler)(nil)`. The right side is the zero value of the type (`nil` for pointers/slices/maps, `T{}` for structs).
- **Receivers vs interfaces.** Value-receiver methods work on values and pointers; pointer-receiver methods need a pointer or addressable value. A map element is not addressable, so `m[k].PtrMethod()` fails.
- **Zero-value mutexes are valid**; never `new(sync.Mutex)`. Keep the mutex as a **named, unexported field** (`mu sync.Mutex`), never embedded, or `Lock`/`Unlock` leak into your public API.
- **Don't embed types in public structs.** Embedding exports the embedded API, freezes your type to it (removing or replacing it later is a breaking change), and muddies docs. Write the few delegating methods by hand, or hold the value in a field.
- **Avoid built-in names** (`error`, `string`, `len`, `copy`, `new`, `close`, `max`, `min`, `any`...) as variables, params, fields or imports. The compiler allows it; shadowing causes latent bugs. `go vet` (shadow) / `predeclared` linters catch it.
- **Start enums at one** (`iota + 1`) so the zero value means "unset", unless the zero value is the genuinely desirable default.
- **Use field tags** (`json:"user_id"`, `yaml:`...) on every marshalled struct field. The serialized form is a contract between systems; tags stop a field rename from silently breaking it.
- **Handle type assertion failures**: always `t, ok := i.(string)`; the single-value form panics.

## Safety at API boundaries

- **Copy slices and maps when you receive or return them** if you store them or they expose internal state. Callers who keep the reference can mutate your struct later.

```go
func (d *Driver) SetTrips(trips []Trip) { d.trips = slices.Clone(trips) }

func (s *Stats) Snapshot() map[string]int {
    s.mu.Lock()
    defer s.mu.Unlock()
    return maps.Clone(s.counters)
}
```

- **Prefer `time` types.** `time.Time` for instants and `time.Duration` for periods, in APIs, flags, config. Use `t.AddDate(0, 0, 1)` for "same time next calendar day" and `t.Add(24*time.Hour)` for "exactly 24h later"; they differ across DST. If an external format can't carry `Duration`, use `int`/`float64` with the unit in the name (`IntervalMillis`); if it can't carry `Time`, use an RFC 3339 string (`time.RFC3339`).
- **`defer` to clean up** (files, locks, spans). Its overhead is negligible except in nanosecond-scale hot paths, and it prevents leaks on early returns.
- **Don't use mutable globals**; inject the dependency (including function pointers like `var timeNow = time.Now`; put a `now func() time.Time` field on the struct instead).

## Errors and panics

- **Pick the error form by what the caller needs:**

| Caller must match it? | Message | Use |
|---|---|---|
| no | static | `errors.New("...")` |
| no | dynamic | `fmt.Errorf("... %d", x)` |
| yes | static | exported `var ErrFoo = errors.New(...)` + `errors.Is` |
| yes | dynamic | custom type (`type NotFoundError struct{...}`) + `errors.As` |

  Exported error vars and types become part of your package's public API.
- **Wrapping:** return the error as-is when you have nothing to add; otherwise `fmt.Errorf("get user %q: %w", id, err)`. Use `%w` when callers may legitimately inspect the cause (document and test that contract for known sentinels/types); use `%v` to deliberately hide the cause so you can change internals later.
- **Keep context short; drop "failed to".** Prefixes like "failed to" stack up ("failed to x: failed to y: ...") as errors bubble. Write `new store: connection refused`, not `failed to create new store: failed to connect: ...`.
- **Naming:** exported error vars `ErrFoo`, unexported `errFoo` (this supersedes the `_` global prefix rule); custom types end in `Error`.
- **Handle each error once.** Options: match with `errors.Is/As` and branch; log and degrade gracefully; return a domain error; or wrap and return. Never log-and-return, since callers up the stack will log it again.
- **Don't panic in production paths.** Return errors. Panic only for truly unrecoverable states or failed startup initialization. Panic/recover is not error handling. In tests use `t.Fatal`/`t.FailNow`, not `panic`.

## Concurrency

- **Don't fire-and-forget goroutines.** Every goroutine needs a predictable end *or* a stop signal, and the owner must be able to wait for it (`sync.WaitGroup`, or a `done` channel closed by the goroutine). Test packages that spawn goroutines with `go.uber.org/goleak` (`goleak.VerifyNone(t)` / `VerifyTestMain`).

```go
stop, done := make(chan struct{}), make(chan struct{})
go func() {
    defer close(done)
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C: flush()
        case <-stop: return
        }
    }
}()
// shutdown: close(stop); <-done
```

- **No goroutines in `init()`**, and no goroutine started as a side effect of importing a package. Expose an object with `Close`/`Stop`/`Shutdown` that stops the goroutine **and waits** for it.
- **`wg.Go(fn)`** (Go 1.25+) replaces the `wg.Add(1); go func(){ defer wg.Done() ... }()` dance; on older Go keep `Add` before `go`.
- **Channel size is one or none.** Default to unbuffered; size 1 is fine. Any larger buffer needs a justification: what bounds it, what happens when it fills under load, who blocks. (Bounded worker-pool job queues are a legitimate exception when the size is derived and documented.)
- **Prefer typed atomics.** Raw `int32`/`int64` with `sync/atomic` functions make it easy to forget the atomic access somewhere. Uber uses `go.uber.org/atomic`; on Go 1.19+ the stdlib `atomic.Int64`, `atomic.Bool`, `atomic.Pointer[T]` give the same safety without a dependency.

## Program structure

- **Exit only in `main()`.** `os.Exit` / `log.Fatal*` anywhere else skips `defer`s, makes control flow opaque, and kills the test binary. Everything else returns errors.
- **Exit at most once**; push the work into `run() error` so `main` is just the exit point and the logic is testable:

```go
func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}

func run() error { /* parse args, build deps, serve; return errors */ }
```

  `run` may take `os.Args[1:]`, or return an exit code if the program has specific codes.
- **Avoid `init()`.** If you can't, it must be deterministic; independent of other `init` ordering; free of environment, working-directory, flag and global-state access; and free of I/O. Acceptable: complex values that can't be a single assignment, pluggable registrations (`database/sql` drivers, codecs), deterministic precomputation. Otherwise move the work into `main`/constructors.

## Performance (hot paths only)

Apply these where profiling shows they matter; elsewhere prefer clarity.

- `strconv.Itoa` / `ParseInt` over `fmt.Sprint` / `Sscanf` for primitives.
- Convert a fixed string to `[]byte` once, outside the loop, and reuse it.
- Give containers a capacity: `make([]T, 0, n)` when appending (a real allocation up front, zero reallocation until full); `make(map[K]V, n)` is only a hint (it right-sizes buckets, may still allocate).

## Style and layout

- **Be consistent**, above all. Apply a convention across a whole package, not file by file.
- **Line length:** soft limit of 99 characters; wrap before it, but it is not enforced.
- **Group** related `const`/`var`/`type` declarations in blocks; don't group unrelated ones. Adjacent local `var`s in a function may be grouped even if unrelated.
- **Imports:** two groups, standard library then everything else (the `goimports` default). Alias only when the package name differs from the last path element or on a real conflict.
- **Package names:** lowercase, no underscores, short, singular (`net/url`), and not `common`/`util`/`shared`/`lib`; avoid names that callers must alias at most sites.
- **Function names:** MixedCaps. Test functions may use underscores to group cases: `TestMyFunction_WhatIsBeingTested`.
- **Order functions by rough call order and group by receiver.** Types and constants first, then `NewT` right after `T`, then exported methods, unexported helpers and plain utility functions near the end.
- **Reduce nesting:** handle errors and special cases first and `return`/`continue`; drop the `else` when the `if` body returns. Initialise with a default then override instead of `if/else` assignment:

```go
a := 10
if b { a = 100 }
```

- **Reduce variable scope:** `if err := f(); err != nil { ... }` unless the value is needed after the `if`. Don't narrow scope if it forces a nested `else` or extra nesting.
- **Top-level vars:** omit the type when the right side states it (`var _s = F()`), keep it when it differs (`var _e error = F()` when `F` returns a concrete type).
- **Unexported top-level `var`/`const` get a `_` prefix** (`_defaultPort`) so they can't be confused with locals. Exception: unexported error vars use `errFoo`.
- **Embedding in structs** (private ones, when you do embed): embedded fields go at the top, separated from regular fields by a blank line; embed only when it provides concrete benefit and wouldn't leak or change semantics.
- **Local declarations:** `:=` when assigning a value, `var x T` when the zero value is the intent (`var s []string`, `var mu sync.Mutex`).
- **`nil` is a valid slice:** return `nil`, not `[]T{}`; check emptiness with `len(s) == 0`, never `s == nil`; `var s []int` is usable with `append`. A nil slice marshals to JSON `null` while `[]T{}` marshals to `[]`, so initialise explicitly for APIs where clients rely on `[]`.
- **Avoid naked parameters** (`printInfo("foo", true, true)`): add `/* isLocal */` comments, or better, replace bool params with custom types or an options struct.
- **Raw string literals** (backticks) to avoid escaping.
- **Struct initialisation:** always use field names (except tiny table-test structs of 3 or fewer fields); omit zero-value fields unless the zero is meaningful for clarity (table tests); `var u User` for an all-zero struct, `&T{...}` instead of `new(T)` with later field assignments.
- **Maps:** `make(map[K]V)` for empty/programmatic fill, a literal when you have fixed initial pairs.
- **Format strings:** declare `printf`-style format strings outside the call as `const` so `go vet` can check them. Name custom printf wrappers with an `f` suffix (`Wrapf`) so vet recognises them.

## Testing and options

- **Table-driven tests** for repeated logic (`tests := []struct{ give, want string }{...}`; `give`/`want` prefixes read well; `tt` as the loop variable; `t.Run(tt.name, ...)`). **Keep tables simple.** When cases need conditional assertions, branching setup, or mock-toggle flags in the struct, split into separate test functions or smaller tables; a test table that needs several `shouldCallX bool` flags or `setupMocks func(...)` fields has outgrown the pattern (a single `shouldErr` flag on a short, straightforward body is fine). Parallel subtests are fine; on Go < 1.22 capture the loop variable.
- **Functional options** for constructors with optional, extensible arguments, especially public APIs likely to grow: an unexported `options` struct, an `Option` interface (or `func(*options)`), and `WithX` constructors. Required parameters stay positional. Prefer the pattern over a growing list of constructor overloads, and over config structs when most callers need zero or one setting.

## Tooling and where Uber is dated

- Uber recommends: `goimports` on save; `go vet`; `errcheck`; `staticcheck`; `revive` (`golint` is deprecated, use `revive`/`staticcheck`); lint through `golangci-lint` (see `assets/golangci.yml`).
- **`go.uber.org/atomic`**: prefer stdlib typed atomics (Go 1.19+) unless you need its extra helpers.
- **`golint`**: archived; use `revive`.
- The guide targets "the two most recent Go minors" and some examples predate Go 1.22 per-iteration loop variables and Go 1.21 `slices`/`maps`/`min`/`max`; modern equivalents are preferred when `go.mod` allows.
