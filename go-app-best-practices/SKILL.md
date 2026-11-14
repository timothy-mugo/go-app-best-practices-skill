---
name: go-app-best-practices
description: Production-grade best practices for writing, structuring, reviewing, and hardening Go (Golang) applications - project layout, idiomatic code, error handling, context propagation, goroutine lifecycle, HTTP servers and APIs, database access, caching, security, observability, testing, and CI/linting. Use this skill whenever the user is working in Go (.go files, go.mod, goroutines, channels, net/http, gin, echo, chi, fiber, sqlx, pgx, gorm, slog, zap) or asks to build, scaffold, debug, review, refactor, or secure a backend service, REST/GraphQL API, worker, or CLI in Go, even if they only say "build a service" or "review this handler" inside a Go repo and never mention best practices.
---

# Go Application Best Practices

A router plus a short set of non-negotiable rules. The depth lives in `references/`
(curated, Go-only). Load only the references the current task needs; they are large.

## Workflow

1. **Classify the task** with the table below and read the matching reference(s) *before*
   writing code. The references contain tested patterns (graceful shutdown, repository +
   transaction handling, state machines, rate limiters) that are easy to get subtly wrong
   from memory.
2. **Check the project first.** Read `go.mod` for the Go version and existing dependencies,
   and follow the layout and libraries already in use. Match the repo; don't introduce a
   second router, logger, or ORM because a reference used a different one.
3. **Write code that satisfies the rules below.**
4. **Verify** (when a Go toolchain is available): `gofmt -l .`, `go vet ./...`,
   `go build ./...`, `go test -race ./...`. Report what you actually ran and its output;
   if you couldn't run something, say so. `assets/` has a `Makefile` and `golangci.yml` to
   copy when the project lacks them.

## Which reference to read

| The task involves... | Read |
|---|---|
| New project, package layout, naming, interfaces, slices/maps/strings, goroutines, channels, mutexes | `references/go-core-idioms.md` |
| Error handling, test setup, benchmarks, CI, linting, generics/modern Go, profiling and allocation tuning | `references/go-errors-testing-tooling.md` |
| Worker pools, fan-out/fan-in, circuit breakers, generics-based abstractions, pooling, service-to-service patterns | `references/go-advanced-patterns.md` |
| HTTP handlers, routing, pagination, versioning, REST/GraphQL/OpenAPI, event-driven design | `references/api-design-patterns.md` |
| Authn/authz, JWT, input validation, CORS/headers, audit logging, secrets, PII, anything money-adjacent | `references/api-security-patterns.md` |
| Reviewing for vulnerabilities; SQLi, XSS, CSRF, SSRF, uploads, deserialization, session attacks | `references/web-security-attacks-guide.md` |
| SQL access, schema design, migrations, transactions, indexing, pool sizing, sharding | `references/database-engineering-patterns.md` |
| Redis, cache invalidation, CDN headers, rate limiting, throttling, latency work | `references/caching-performance-patterns.md` |
| Login flows, sessions, forms, WebSockets, server-side state | `references/web-application-patterns.md` |
| Payments, order lifecycles, state machines, background jobs, retries, idempotency | `references/real-world-project-patterns.md` |
| Table-driven tests, mocks/fakes, integration tests, fuzzing, benchmarks | `references/testing-patterns.md` |
| `io.Reader`/`Writer` composition, `net/http` server/client details, middleware, `database/sql` basics | `references/standard-library/io-and-http.md` |
| `encoding/json`, `strings`/`strconv`, `regexp`, `filepath` vs `path`, `fmt` verbs and `Stringer` | `references/standard-library/encoding-and-strings.md` |
| `time.Duration`, timers/tickers, `log/slog`, `crypto/rand` and constant-time compare | `references/standard-library/time-and-context.md` |
| Logging, metrics, tracing, health/readiness endpoints | `references/observability-patterns.md` |

The `standard-library/` files are short (about 170 lines each) and cover the stdlib idioms every
service touches; read the one matching the packages you're about to use.

Most real tasks span two or three rows (an order-creation endpoint touches API design, database,
security, and real-world patterns). Read each relevant one; skim the Contents list at the top
of long files and jump to the section you need rather than reading end to end.

## Non-negotiable rules

Each rule is here because the failure is silent or expensive in production, not because
of style. Following the reasoning matters more than following the letter.

### Errors
- **Never discard an error.** If ignoring is truly correct, write `_ = f()` with a comment saying why. A swallowed error becomes a corrupted state discovered three layers later.
- **Wrap with context using `%w`**: `fmt.Errorf("create order %s: %w", id, err)`. Callers can then use `errors.Is` / `errors.As`; `%v` or string comparison destroys that.
- **Handle each error once**: either return it or log it, not both. Log at the boundary (handler, worker loop, `main`), otherwise one failure produces a dozen duplicate log lines.
- Use sentinel errors (`var ErrNotFound = errors.New(...)`) or typed errors for conditions callers must branch on, and map them to HTTP/gRPC status codes in one place.
- `panic` is for programmer bugs at startup, not for runtime conditions. Recover at goroutine and request boundaries so one bad request can't kill the process.
- Don't shadow `err` with `:=` inside an `if`/`for` block when you meant to assign the outer one; the inner error is silently lost.

### Context
- Take `ctx context.Context` as the **first parameter** of anything that does I/O or can block, and pass it all the way down: `http.NewRequestWithContext`, `db.QueryContext`, `ExecContext`, `BeginTx`. Without it, a cancelled client request keeps consuming DB connections and goroutines.
- Always `defer cancel()` right after `context.WithTimeout` / `WithCancel`. A missing cancel leaks timers and children.
- Don't store a context in a struct, and don't put business data in context values (request-scoped metadata such as request IDs and auth principals only).

### Goroutines and concurrency
- Before writing `go`, know **who stops it and how**. Every goroutine needs a lifecycle: context cancellation, a closed channel, or a `WaitGroup`/`errgroup` that the owner waits on. Leaked goroutines are the most common Go memory leak.
- Call `wg.Add(n)` **before** `go`, never inside the goroutine; otherwise `Wait()` can return early. Prefer `golang.org/x/sync/errgroup` when tasks can fail, since it gives you first-error cancellation for free.
- Only the sender closes a channel; never close from the receiver or close twice.
- Guard shared state with a mutex or confine it to one goroutine. Don't copy a `sync.Mutex`/`WaitGroup` (pass pointers). Release locks with `defer mu.Unlock()`.
- Loop variables are per-iteration since Go 1.22 (check `go.mod`'s `go` line); older code needs the `v := v` copy before capturing in a closure.
- In modules targeting Go < 1.23, avoid `time.After` in a loop (each call leaks a timer until it fires); use one `time.NewTimer` with `Reset`/`Stop`. From 1.23 the leak is fixed, so don't rewrite working code. Always `defer ticker.Stop()` for tickers. Use `time.Duration` constants (`30 * time.Second`), never bare ints.
- Bound concurrency (worker pool or semaphore). Unbounded `go` per request is a denial-of-service you wrote yourself.

### HTTP servers and clients
- **Never use `http.ListenAndServe` or `http.DefaultClient` bare.** They have no timeouts, so a slow client or upstream pins resources forever. Configure `http.Server{ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout}` and `http.Client{Timeout}` (or per-request contexts).
- **Graceful shutdown**: `signal.NotifyContext` for SIGINT/SIGTERM, then `srv.Shutdown(ctx)` with a deadline, then close DB/queues in reverse order of creation. Otherwise every deploy drops in-flight requests.
- Create your own `http.NewServeMux()`; `DefaultServeMux` is a global any imported package can register handlers on. Go 1.22+ patterns (`mux.HandleFunc("GET /users/{id}", ...)`, `r.PathValue("id")`) remove the need for a router dependency in most services.
- Always `defer resp.Body.Close()` after checking the error, and drain/limit bodies you don't fully read. `http.Client` does not treat 4xx/5xx as errors, so check `resp.StatusCode` yourself. Cap request bodies with `http.MaxBytesReader` and untrusted response bodies with `io.LimitReader`.
- Decode JSON strictly where it matters (`DisallowUnknownFields`), validate every field, return consistent error JSON, and never leak internal error text to clients.
- Handlers should be thin: parse and validate, call a service, map the result or error to a response. Business logic in handlers can't be tested without HTTP.

### Database
- **Parameterised queries only**, never string-built SQL (`$1`, `?`, or a query builder). This is the SQL-injection fix; there's no safe sanitising alternative.
- Use `*Context` methods, `defer rows.Close()`, and check `rows.Err()` after iteration.
- Transactions: `defer tx.Rollback()` immediately after `BeginTx` (a no-op after commit), keep them short, never make network calls inside, and make multi-step writes (create order plus reserve stock) atomic.
- Configure the pool (`SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime`); the defaults are unbounded and will exhaust Postgres under load.
- Schema changes go through versioned, reviewed migrations, not ad-hoc DDL in app startup.

### Structure and design
- Layout: `cmd/<app>/main.go` (wiring only), `internal/` for everything private (`internal/<domain>/...` by feature or domain, not by `models/` `utils/` `helpers/` layers), `pkg/` only for code truly meant to be imported by others.
- **Accept interfaces, return structs.** Define interfaces where they are *consumed*, keep them small (1-3 methods), and don't create an interface until there is a second implementation or a test seam that needs one.
- Dependency injection through constructors (`NewService(repo, logger)`); no package-level mutable state, no `init()` side effects. It keeps tests hermetic and wiring visible in `main`.
- Configuration comes from environment/flags, is parsed and **validated once at startup**, and fails fast. Secrets never live in code, images, or logs.
- Pre-allocate slices/maps when the size is known (`make([]T, 0, n)`). Writing to a nil map panics. `defer` inside a loop runs at function end, so close per-iteration resources in a helper function.
- Prefer the standard library (`net/http` with Go 1.22 pattern routing, `log/slog`, `errors`, `encoding/json`) and add a dependency only for a clear gain.

### Security
- Treat all external input as hostile: validate type, length, range, and format at the boundary; allowlist rather than blocklist.
- Hash passwords with argon2id or bcrypt; compare secrets with `subtle.ConstantTimeCompare`; use `crypto/rand` (never `math/rand`; `rand.Text()` on Go 1.24+) for tokens and IDs, and never set `InsecureSkipVerify` outside tests.
- Authorisation is checked on **every** object access (no trusting IDs from the client), which is the broken-object-level-authorisation class of bugs.
- Set security headers, restrictive CORS, and rate limits on auth and expensive endpoints. Don't log credentials, tokens, or PII.
- Run `govulncheck ./...` and keep dependencies updated. When touching auth, uploads, outbound fetches (SSRF), or anything money-related, read `api-security-patterns.md` and `web-security-attacks-guide.md` first.

### Standard library idioms
- `encoding/json`: use `json.NewDecoder(r)` / `NewEncoder(w)` on streams instead of `ReadAll` + `Unmarshal`; a nil slice marshals to `null` and an empty one to `[]`, so initialise slices in API responses that clients iterate; tag internal fields `json:"-"`.
- Prefer `strconv` over `fmt.Sprintf` for primitive conversions (faster, fewer allocations) and `strings.Builder` with `Grow` for concatenation in loops.
- Compile regexes once at package level with `regexp.MustCompile`; use `Compile` only for patterns derived from input. Go's engine is linear-time, so it has no ReDoS but also no backreferences.
- `path/filepath` for OS paths, `path` for URLs and logical paths; never mix them.
- A `String()` method that formats its own receiver with `%s`/`%v` recurses forever; convert to the underlying type first.
- Pass `*slog.Logger` as an explicit dependency rather than reaching for globals.

### Observability
- Use structured logging with `log/slog` (JSON in production), carrying a request ID and the relevant entity IDs. Free-form `Printf` logs can't be queried.
- Expose `/healthz` (liveness: process is up) and `/readyz` (readiness: dependencies reachable) separately; orchestrators treat them differently.
- Emit RED metrics (rate, errors, duration) per route and propagate trace context across outbound calls.

### Testing
- Table-driven tests with `t.Run` subtests, `t.Helper()` in helpers, `t.Cleanup` for teardown, and `t.Parallel()` where safe.
- Always run `go test -race ./...` - data races only show up there.
- Test through small interfaces with hand-written fakes; use `httptest` for handlers; use real dependencies (testcontainers or a test DB) for repository tests since mocks can't catch SQL mistakes.
- Cover the error paths and boundaries (empty, nil, negative, huge, cancelled context), not just the happy path. Use fuzzing for parsers and validators.

## Reviewing existing Go code

When asked to review or audit rather than write: walk the rules above as a checklist against
the code, and report findings ordered by severity (data race / leak / injection / swallowed
error first, naming and style last). For each finding give the location, why it fails in
practice (the concrete scenario), and a corrected snippet. Don't pad the review with nits
when real defects exist; a short list of real problems is more useful.

## Pre-commit checklist

- [ ] Every error handled, wrapped with `%w`, or explicitly discarded with a reason
- [ ] Every goroutine has an owner and a stop condition; `wg.Add` precedes `go`
- [ ] `ctx` flows from the request to every DB/HTTP/RPC call; cancels are deferred
- [ ] Server and client timeouts set; graceful shutdown wired in `main`
- [ ] Resources closed with `defer` (bodies, rows, files, tx rollback)
- [ ] SQL parameterised; input validated; no secrets in code or logs
- [ ] Tests cover error paths and pass with `-race`; `gofmt`, `go vet`, linter clean
- [ ] Health endpoints, structured logs, and metrics present for anything deployed

## Attribution

The references and rules in this skill are adapted from [nutcas3/best-practices](https://github.com/nutcas3/best-practices). The `references/standard-library/` files come from the sw-factory agentic-orchestrator Go guidelines.
