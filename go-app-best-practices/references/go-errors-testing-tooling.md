# Go Reliability: Errors, Testing, Performance, CI/CD, Linting, Modern Go

> Source: adapted from [nutcas3/best-practices](https://github.com/nutcas3/best-practices).

Sections 5, 6, 8-12 of the Go best-practices guide plus the pre-commit/production checklist.

## Contents

- [5. Error Management & Reliability](#5-error-management--reliability)
- [6. Testing & Production Readiness](#6-testing--production-readiness)
- [8. Performance Optimization](#8-performance-optimization)
- [9. CI/CD Configuration](#9-cicd-configuration)
- [10. Linter Configuration](#10-linter-configuration)
- [11. Modern Go Features (Go 1.18+)](#11-modern-go-features-go-118)
- [12. Advanced Performance Patterns](#12-advanced-performance-patterns)
- [Summary Checklist](#summary-checklist)

## 5. Error Management & Reliability

### Error Wrapping

```go
// ❌ BAD: Lost error context
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err  // Lost information about what we were doing
    }
    // ...
}

// ✅ GOOD: Wrapped errors with context
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("load config from %s: %w", path, err)
    }
    
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    
    return &cfg, nil
}

// Usage: Can inspect wrapped errors
cfg, err := LoadConfig("config.json")
if err != nil {
    if errors.Is(err, os.ErrNotExist) {
        // Handle missing file specifically
    }
    return err
}
```

### Custom Error Types

```go
// ✅ GOOD: Structured errors
type ValidationError struct {
    Field string
    Value interface{}
    Msg   string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Msg)
}

func ValidateUser(u *User) error {
    if u.Email == "" {
        return &ValidationError{
            Field: "email",
            Value: u.Email,
            Msg:   "email is required",
        }
    }
    return nil
}

// Usage
if err := ValidateUser(user); err != nil {
    var valErr *ValidationError
    if errors.As(err, &valErr) {
        // Handle validation error specifically
        log.Printf("Invalid field: %s", valErr.Field)
    }
}
```

### Don't Handle Errors Twice

```go
// ❌ BAD: Logging and returning
func ProcessOrder(id string) error {
    order, err := fetchOrder(id)
    if err != nil {
        log.Printf("Failed to fetch order: %v", err)  // Logged here
        return err  // AND returned - caller might log again!
    }
    return nil
}

// ✅ GOOD: Return error, let caller decide
func ProcessOrder(id string) error {
    order, err := fetchOrder(id)
    if err != nil {
        return fmt.Errorf("fetch order %s: %w", id, err)
    }
    return nil
}

// Caller logs once at appropriate level
if err := ProcessOrder(orderID); err != nil {
    log.Printf("Process order failed: %v", err)
}
```

### Resource Management with defer

```go
// ✅ GOOD: Always defer Close() and check errors
func ReadConfig(path string) (*Config, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()  // Guaranteed to run
    
    var cfg Config
    if err := json.NewDecoder(f).Decode(&cfg); err != nil {
        return nil, err
    }
    
    return &cfg, nil
}

// ✅ GOOD: Check Close() error for writes
func WriteConfig(path string, cfg *Config) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer func() {
        if cerr := f.Close(); cerr != nil && err == nil {
            err = cerr  // Return close error if no other error
        }
    }()
    
    return json.NewEncoder(f).Encode(cfg)
}

// ✅ GOOD: HTTP response body must be closed
func FetchData(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()  // Critical: prevents connection leak
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }
    
    return io.ReadAll(resp.Body)
}
```

### Use time.Duration, Not Integers

```go
// ❌ BAD: Ambiguous units
func SetTimeout(timeout int) {
    // Is this seconds? Milliseconds? Minutes?
}

// ✅ GOOD: Explicit duration type
func SetTimeout(timeout time.Duration) {
    // Clear and type-safe
}

// Usage
SetTimeout(30 * time.Second)
SetTimeout(500 * time.Millisecond)
```

---

## 6. Testing & Production Readiness

### Table-Driven Tests

```go
// ✅ GOOD: Standard Go testing pattern
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {
            name:    "valid email",
            email:   "user@example.com",
            wantErr: false,
        },
        {
            name:    "missing @",
            email:   "userexample.com",
            wantErr: true,
        },
        {
            name:    "empty email",
            email:   "",
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.email)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Race Detection

```bash
# Always run tests with race detector
go test -race ./...

# Run specific package
go test -race ./internal/service

# With coverage
go test -race -coverprofile=coverage.out ./...
```

### Production HTTP Clients

```go
// ❌ BAD: Default client has no timeouts
resp, err := http.Get("https://api.example.com/data")

// ✅ GOOD: Custom client with timeouts
var httpClient = &http.Client{
    Timeout: 10 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    },
}

func FetchData(url string) ([]byte, error) {
    resp, err := httpClient.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    return io.ReadAll(resp.Body)
}
```

### Production HTTP Servers

```go
// ❌ BAD: Default server has no timeouts
http.ListenAndServe(":8080", handler)

// ✅ GOOD: Configured server with timeouts
server := &http.Server{
    Addr:         ":8080",
    Handler:      handler,
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
}

if err := server.ListenAndServe(); err != nil {
    log.Fatal(err)
}
```

### Graceful Shutdown

```go
func main() {
    server := &http.Server{
        Addr:    ":8080",
        Handler: setupRoutes(),
    }
    
    // Start server in goroutine
    go func() {
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()
    
    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit
    
    log.Println("Shutting down server...")
    
    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }
    
    log.Println("Server exited")
}
```

---

## 8. Performance Optimization

### String Concatenation

```go
// ❌ BAD: String concatenation in loop
func BuildQuery(fields []string) string {
    query := "SELECT "
    for i, field := range fields {
        query += field  // Creates new string each iteration
        if i < len(fields)-1 {
            query += ", "
        }
    }
    return query
}

// ✅ GOOD: Use strings.Builder
func BuildQuery(fields []string) string {
    var b strings.Builder
    b.WriteString("SELECT ")
    for i, field := range fields {
        b.WriteString(field)
        if i < len(fields)-1 {
            b.WriteString(", ")
        }
    }
    return b.String()
}
```

### Avoid Unnecessary Allocations

```go
// ❌ BAD: Allocates on every call
func GetDefaultConfig() Config {
    return Config{
        Timeout:  30 * time.Second,
        MaxRetry: 3,
    }
}

// ✅ GOOD: Reuse constant
var defaultConfig = Config{
    Timeout:  30 * time.Second,
    MaxRetry: 3,
}

func GetDefaultConfig() Config {
    return defaultConfig
}
```

---

## 9. CI/CD Configuration

### GitHub Actions Workflow

```yaml
name: Go CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.22'
        cache: true
    
    - name: Download dependencies
      run: go mod download
    
    - name: Verify dependencies
      run: go mod verify
    
    - name: Run go vet
      run: go vet ./...
    
    - name: Run golangci-lint
      uses: golangci/golangci-lint-action@v4
      with:
        version: latest
        args: --timeout=5m
    
    - name: Run tests with race detector
      run: go test -v -race -coverprofile=coverage.out ./...
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
    
    - name: Run integration tests
      run: go test -v -tags=integration ./test/integration/...
      env:
        DATABASE_URL: postgres://test:test@localhost:5432/testdb

  build:
    name: Build
    runs-on: ubuntu-latest
    needs: test
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.22'
    
    - name: Build binaries
      run: |
        go build -v -o bin/api ./cmd/api
        go build -v -o bin/worker ./cmd/worker
    
    - name: Upload artifacts
      uses: actions/upload-artifact@v3
      with:
        name: binaries
        path: bin/
```

---

## 10. Linter Configuration

### .golangci.yml

```yaml
run:
  timeout: 5m
  tests: true
  modules-download-mode: readonly

linters:
  enable:
    - errcheck      # Check for unchecked errors
    - gosimple      # Simplify code
    - govet         # Standard Go analyzer
    - ineffassign   # Detect ineffectual assignments
    - staticcheck   # Advanced static analysis
    - unused        # Find unused code
    - gofmt         # Check formatting
    - goimports     # Check imports
    - misspell      # Fix spelling mistakes
    - revive        # Fast, configurable linter
    - gosec         # Security issues
    - bodyclose     # Check HTTP body closed
    - noctx         # Find HTTP requests without context
    - errname       # Check error naming
    - errorlint     # Find error wrapping issues

linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: true
  
  govet:
    check-shadowing: true
  
  revive:
    rules:
      - name: exported
        severity: warning
      - name: unexported-return
        severity: warning
      - name: var-naming
        severity: warning

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
        - gosec
```

---

## 11. Modern Go Features (Go 1.18+)

### Using `any` Instead of `interface{}`

In Go 1.18 and later, `any` is a type alias for the empty interface. It's more readable and should be preferred in new code.

```go
// ❌ BAD: Using empty interface
func ProcessData(data interface{}) {}
var config map[string]interface{}

// ✅ GOOD: Using any
func ProcessData(data any) {}
var config map[string]any
```

### Type Parameters (Generics)

```go
// ✅ GOOD: Generic function with constraints
func Min[T constraints.Ordered](x, y T) T {
    if x < y {
        return x
    }
    return y
}

// ✅ GOOD: Generic data structure
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(item T) {
    s.items = append(s.items, item)
}

func (s *Stack[T]) Pop() (T, bool) {
    var zero T
    if len(s.items) == 0 {
        return zero, false
    }
    item := s.items[len(s.items)-1]
    s.items = s.items[:len(s.items)-1]
    return item, true
}

// Usage
ints := &Stack[int]{}
ints.Push(1)
ints.Push(2)
if val, ok := ints.Pop(); ok {
    fmt.Println(val) // 2
}
```

### Type Sets and Interface Satisfaction

```go
// ✅ GOOD: Type set with union
type Number interface {
    ~int | ~int32 | ~int64 | ~float32 | ~float64
}

// ✅ GOOD: Type constraint with methods
type Stringer interface {
    String() string
}

type Printable interface {
    Number | Stringer
}

func Print[T Printable](x T) {
    fmt.Println(x)
}
```

### Structured Logging with `slog` (Go 1.21+)

```go
// ✅ GOOD: Structured logging
var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func ProcessOrder(ctx context.Context, orderID string) error {
    logger.Info("processing order",
        "order_id", orderID,
        "user_id", ctx.Value("user_id"),
    )
    
    // Process order...
    
    logger.Debug("order processed",
        "order_id", orderID,
        "duration", time.Since(start),
    )
    return nil
}
```

### Working with Slices (Go 1.21+)

```go
// ✅ GOOD: Using new slice package
import "slices"

// Sort
slices.Sort(items)

// Binary search
index := slices.BinarySearch(sorted, target)

// Compare
if slices.Equal(a, b) {
    // Slices have same elements
}

// Clone
copy := slices.Clone(original)
```

### Maps (Go 1.21+)

```go
// ✅ GOOD: Using new maps package
import "maps"

// Copy map
dest := maps.Clone(source)

// Compare maps
if maps.Equal(a, b) {
    // Maps have same key-value pairs
}

// Clear map
maps.Clear(m)  // Better than making a new map
```

### Error Handling with `errors.Join` (Go 1.20+)

```go
// ✅ GOOD: Combining multiple errors
func ProcessItems(items []Item) error {
    var errs []error
    for _, item := range items {
        if err := process(item); err != nil {
            errs = append(errs, fmt.Errorf("process %v: %w", item.ID, err))
        }
    }
    return errors.Join(errs...)
}
```

## 12. Advanced Performance Patterns

### Optimistic Special Cases

```go
// ❌ BAD: Always using the complex general case
func ProcessItems(items []Item) error {
    var result []Result
    for _, item := range items {
        // Complex processing...
    }
    return saveResults(result)
}

// ✅ GOOD: Handle common special cases first
func ProcessItems(items []Item) error {
    // Handle empty case
    if len(items) == 0 {
        return nil
    }

    // Handle single item case
    if len(items) == 1 {
        return processSingleItem(items[0])
    }

    // Handle general case
    var result []Result
    for _, item := range items {
        // Complex processing...
    }
    return saveResults(result)
}
```

### Smart Caching Patterns

```go
// ✅ GOOD: Local cache for high-locality lookups
type Cache struct {
    mu    sync.RWMutex
    items map[string]*Item
    // Small LRU cache for hot items
    hot   *lru.Cache
}

func (c *Cache) Get(key string) (*Item, error) {
    // Try hot cache first
    if val, ok := c.hot.Get(key); ok {
        return val.(*Item), nil
    }

    // Fall back to main storage
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    if item, ok := c.items[key]; ok {
        // Add to hot cache
        c.hot.Add(key, item)
        return item, nil
    }
    
    return nil, ErrNotFound
}
```

### Memory Layout Optimization

```go
// ❌ BAD: Cache-unfriendly struct layout
type BadLayout struct {
    flag    bool       // 1 byte + 7 padding
    data    []byte     // 24 bytes
    counter int64      // 8 bytes
    active  bool       // 1 byte + 7 padding
}  // Total: 48 bytes

// ✅ GOOD: Cache-friendly struct layout
type GoodLayout struct {
    data    []byte     // 24 bytes
    counter int64      // 8 bytes
    flag    bool       // 1 byte
    active  bool       // 1 byte
    // 6 bytes padding, but better than 14
}  // Total: 40 bytes
```

### Lazy Computation

```go
// ❌ BAD: Eager computation
type Config struct {
    data      []byte
    hash      []byte // Pre-computed hash
}

func NewConfig(data []byte) *Config {
    return &Config{
        data: data,
        hash: calculateHash(data), // Always computed
    }
}

// ✅ GOOD: Lazy computation
type Config struct {
    data      []byte
    hash      []byte
    hashOnce  sync.Once
}

func (c *Config) Hash() []byte {
    c.hashOnce.Do(func() {
        c.hash = calculateHash(c.data)
    })
    return c.hash
}
```

### Buffer Pooling

```go
// ✅ GOOD: Reuse buffers for reduced allocations
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func ProcessLargeData(data []byte) error {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()
    
    // Use buffer...
    return nil
}
```

### Batch Processing

```go
// ❌ BAD: Processing one at a time
for _, item := range items {
    if err := db.Save(item); err != nil {
        return err
    }
}

// ✅ GOOD: Batch processing
const batchSize = 100
for i := 0; i < len(items); i += batchSize {
    end := i + batchSize
    if end > len(items) {
        end = len(items)
    }
    if err := db.SaveBatch(items[i:end]); err != nil {
        return err
    }
}
```

### Custom Memory Management

```go
// ✅ GOOD: Object pool for frequently allocated items
type Pool struct {
    pool sync.Pool
    size int
}

func NewPool(size int) *Pool {
    return &Pool{
        pool: sync.Pool{
            New: func() interface{} {
                return make([]byte, size)
            },
        },
        size: size,
    }
}

func (p *Pool) Get() []byte {
    return p.pool.Get().([]byte)
}

func (p *Pool) Put(buf []byte) {
    if cap(buf) >= p.size {
        p.pool.Put(buf[:p.size])
    }
}
```

## Summary Checklist

### Before Committing Code

- [ ] All errors are handled or explicitly ignored with `_ = err`
- [ ] Resources (files, HTTP bodies, DB rows) are closed with `defer`
- [ ] No goroutines without clear lifecycle management
- [ ] `wg.Add()` called before starting goroutines
- [ ] Context passed through call chains
- [ ] No variable shadowing with `:=`
- [ ] Slices pre-allocated when size is known
- [ ] HTTP clients and servers have timeouts configured
- [ ] Tests pass with `-race` flag
- [ ] Linter passes with no warnings

### Production Deployment

- [ ] Graceful shutdown implemented
- [ ] Structured logging in place
- [ ] Metrics and monitoring configured
- [ ] Health check endpoints exposed
- [ ] Configuration externalized (12-factor app)
- [ ] Secrets managed securely (not in code)
- [ ] Database migrations automated
- [ ] CI/CD pipeline configured
