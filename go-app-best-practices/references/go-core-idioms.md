# Go Core: Structure, Idioms, Concurrency, Types, Interfaces

> Source: adapted from [nutcas3/best-practices](https://github.com/nutcas3/best-practices).

Sections 1-4 and 7 of the Go best-practices guide (original numbering kept).

## Contents

- [1. Project Organization & Structure](#1-project-organization--structure)
- [2. Idiomatic Code & Control Structures](#2-idiomatic-code--control-structures)
- [3. Concurrency Best Practices](#3-concurrency-best-practices)
- [4. Data Types & Memory Management](#4-data-types--memory-management)
- [7. Interface Design](#7-interface-design)

## 1. Project Organization & Structure

### Standard Project Layout

Use the following directory structure for maintainable, scalable Go projects:

```
/project-root
├── cmd/                    # Application entry points (main packages)
│   ├── api/               # API server binary
│   │   └── main.go
│   └── worker/            # Background worker binary
│       └── main.go
├── internal/              # Private application code (cannot be imported externally)
│   ├── domain/           # Business logic and domain models
│   ├── handler/          # HTTP handlers
│   ├── repository/       # Data access layer
│   └── service/          # Business service layer
├── pkg/                   # Public library code (can be imported by external projects)
│   └── logger/           # Reusable logging utilities
├── api/                   # API contracts and definitions
│   ├── openapi/          # OpenAPI/Swagger specs
│   └── proto/            # Protocol Buffer definitions
├── configs/               # Configuration files
│   ├── dev.yaml
│   └── prod.yaml
├── deployments/           # Deployment configurations
│   ├── docker/           # Dockerfiles
│   └── kubernetes/       # K8s manifests
├── scripts/               # Build and deployment scripts
├── test/                  # Additional test data and integration tests
├── .golangci.yml         # Linter configuration
├── go.mod                # Go module definition
├── go.sum                # Dependency checksums
├── Makefile              # Build automation
└── README.md             # Project documentation
```

### Key Principles

**✅ DO:**
- Place all application entry points in `/cmd` with descriptive subdirectories
- Use `/internal` for code that should remain private to your project
- Keep `/pkg` minimal and only for truly reusable libraries
- Organize code by domain/feature, not by technical layer

**❌ DON'T:**
- Create flat structures with all files in the root
- Use generic names like `utils`, `common`, or `helpers` as package names
- Mix business logic with infrastructure concerns
- Export everything by default

### Package Naming Best Practices

```go
// ❌ BAD: Generic, meaningless names
package utils
package helpers
package common

// ✅ GOOD: Descriptive, purpose-driven names
package encoding
package http
package postgres
```

**Rules:**
1. Name packages after what they **provide**, not what they contain
2. Use singular nouns (e.g., `user`, not `users`)
3. Keep names short and lowercase
4. Avoid stuttering: `http.Server`, not `http.HTTPServer`

### Minimize Exports

```go
// ❌ BAD: Everything exported
type UserRepository struct {
    DB *sql.DB
}

func (r *UserRepository) FindByID(id int) (*User, error) { }
func (r *UserRepository) BuildQuery(filters map[string]string) string { }
func (r *UserRepository) ValidateInput(input string) bool { }

// ✅ GOOD: Only essential parts exported
type UserRepository struct {
    db *sql.DB  // unexported
}

func (r *UserRepository) FindByID(id int) (*User, error) { }

// Helper methods unexported
func (r *UserRepository) buildQuery(filters map[string]string) string { }
func (r *UserRepository) validateInput(input string) bool { }
```

---

## 2. Idiomatic Code & Control Structures

### The Happy Path Left-Aligned

Keep successful execution flow aligned to the left by handling errors early:

```go
// ❌ BAD: Deep nesting
func ProcessUser(id int) error {
    user, err := fetchUser(id)
    if err == nil {
        if user.IsActive {
            profile, err := fetchProfile(user.ID)
            if err == nil {
                if profile.IsComplete {
                    return updateUser(user, profile)
                } else {
                    return errors.New("incomplete profile")
                }
            } else {
                return err
            }
        } else {
            return errors.New("inactive user")
        }
    }
    return err
}

// ✅ GOOD: Early returns, happy path left-aligned
func ProcessUser(id int) error {
    user, err := fetchUser(id)
    if err != nil {
        return fmt.Errorf("fetch user: %w", err)
    }
    
    if !user.IsActive {
        return errors.New("inactive user")
    }
    
    profile, err := fetchProfile(user.ID)
    if err != nil {
        return fmt.Errorf("fetch profile: %w", err)
    }
    
    if !profile.IsComplete {
        return errors.New("incomplete profile")
    }
    
    return updateUser(user, profile)
}
```

### Avoid Variable Shadowing

```go
// ❌ BAD: Variable shadowing with :=
func GetConfig() error {
    config, err := loadConfig()
    if err != nil {
        return err
    }
    
    if config.UseCache {
        // This creates a NEW 'config' variable in this scope!
        config, err := loadCacheConfig()
        if err != nil {
            return err
        }
        // Original config is unchanged
        fmt.Println(config.CacheSize)
    }
    
    // Still using the original config here
    return nil
}

// ✅ GOOD: Explicit variable names or reuse
func GetConfig() error {
    config, err := loadConfig()
    if err != nil {
        return err
    }
    
    if config.UseCache {
        cacheConfig, err := loadCacheConfig()
        if err != nil {
            return err
        }
        config.CacheSize = cacheConfig.CacheSize
    }
    
    return nil
}
```

### Functional Options Pattern

For constructors with many optional parameters:

```go
// ❌ BAD: Too many parameters
func NewServer(addr string, timeout int, maxConns int, enableTLS bool, 
               certFile string, keyFile string, logger *log.Logger) *Server {
    // ...
}

// ✅ GOOD: Functional options pattern
type Server struct {
    addr      string
    timeout   time.Duration
    maxConns  int
    enableTLS bool
    certFile  string
    keyFile   string
    logger    *log.Logger
}

type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

func WithTLS(certFile, keyFile string) Option {
    return func(s *Server) {
        s.enableTLS = true
        s.certFile = certFile
        s.keyFile = keyFile
    }
}

func WithLogger(logger *log.Logger) Option {
    return func(s *Server) {
        s.logger = logger
    }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{
        addr:     addr,
        timeout:  30 * time.Second, // defaults
        maxConns: 100,
        logger:   log.Default(),
    }
    
    for _, opt := range opts {
        opt(s)
    }
    
    return s
}

// Usage
server := NewServer(":8080",
    WithTimeout(60*time.Second),
    WithTLS("cert.pem", "key.pem"),
)
```

### Avoid init() Functions

```go
// ❌ BAD: Using init() for state
var db *sql.DB

func init() {
    var err error
    db, err = sql.Open("postgres", "connection-string")
    if err != nil {
        // Can't return error from init()!
        panic(err)
    }
}

// ✅ GOOD: Explicit initialization
type Database struct {
    conn *sql.DB
}

func NewDatabase(connStr string) (*Database, error) {
    conn, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, fmt.Errorf("open database: %w", err)
    }
    
    if err := conn.Ping(); err != nil {
        return nil, fmt.Errorf("ping database: %w", err)
    }
    
    return &Database{conn: conn}, nil
}
```

---

## 3. Concurrency Best Practices

### Goroutine Lifecycle Management

**Golden Rule:** Never start a goroutine without knowing how it will stop.

```go
// ❌ BAD: Goroutine leak - no way to stop
func StartProcessor() {
    go func() {
        for {
            process()
            time.Sleep(time.Second)
        }
    }()
}

// ✅ GOOD: Controlled lifecycle with context
func StartProcessor(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                process()
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

### WaitGroup Best Practices

```go
// ❌ BAD: Race condition - Add() called in goroutine
func ProcessItems(items []Item) {
    var wg sync.WaitGroup
    
    for _, item := range items {
        go func(i Item) {
            wg.Add(1)  // WRONG: May not execute before wg.Wait()
            defer wg.Done()
            process(i)
        }(item)
    }
    
    wg.Wait()
}

// ✅ GOOD: Add() called before starting goroutine
func ProcessItems(items []Item) {
    var wg sync.WaitGroup
    
    for _, item := range items {
        wg.Add(1)  // Correct: Called in parent goroutine
        go func(i Item) {
            defer wg.Done()
            process(i)
        }(item)
    }
    
    wg.Wait()
}
```

### Using errgroup for Error Handling

```go
import "golang.org/x/sync/errgroup"

// ✅ GOOD: Coordinated error handling and cancellation
func FetchMultipleResources(ctx context.Context, ids []string) error {
    g, ctx := errgroup.WithContext(ctx)
    
    results := make([]Resource, len(ids))
    
    for i, id := range ids {
        i, id := i, id // Capture loop variables (only needed before Go 1.22; later versions scope per iteration)
        g.Go(func() error {
            resource, err := fetchResource(ctx, id)
            if err != nil {
                return fmt.Errorf("fetch %s: %w", id, err)
            }
            results[i] = resource
            return nil
        })
    }
    
    // Wait for all goroutines, returns first error if any
    if err := g.Wait(); err != nil {
        return err
    }
    
    return processResults(results)
}
```

### Context Propagation

```go
// ✅ GOOD: Context passed through call chain
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Add timeout for downstream operations
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    result, err := fetchData(ctx)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            http.Error(w, "Request timeout", http.StatusGatewayTimeout)
            return
        }
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(result)
}

func fetchData(ctx context.Context) (*Data, error) {
    // Context propagated to database call
    return db.QueryContext(ctx, "SELECT ...")
}
```

### Safe Map Access

```go
// ❌ BAD: Concurrent map access causes panic
type Cache struct {
    data map[string]string
}

func (c *Cache) Set(key, value string) {
    c.data[key] = value  // PANIC if called concurrently!
}

// ✅ GOOD: Protected with mutex
type Cache struct {
    mu   sync.RWMutex
    data map[string]string
}

func (c *Cache) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = value
}

func (c *Cache) Get(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    val, ok := c.data[key]
    return val, ok
}

// ✅ BETTER: Use sync.Map for high-concurrency scenarios
type Cache struct {
    data sync.Map
}

func (c *Cache) Set(key, value string) {
    c.data.Store(key, value)
}

func (c *Cache) Get(key string) (string, bool) {
    val, ok := c.data.Load(key)
    if !ok {
        return "", false
    }
    return val.(string), true
}
```

### Never Copy Sync Types

```go
// ❌ BAD: Copying mutex breaks synchronization
type Counter struct {
    mu    sync.Mutex
    count int
}

func (c Counter) Increment() {  // Receiver by value - copies mutex!
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}

// ✅ GOOD: Pointer receiver
func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}
```

---

## 4. Data Types & Memory Management

### Slice Pre-allocation

```go
// ❌ BAD: Repeated allocations
func ProcessItems(n int) []Result {
    var results []Result
    for i := 0; i < n; i++ {
        results = append(results, process(i))  // May reallocate multiple times
    }
    return results
}

// ✅ GOOD: Pre-allocated capacity
func ProcessItems(n int) []Result {
    results := make([]Result, 0, n)  // Allocate once
    for i := 0; i < n; i++ {
        results = append(results, process(i))
    }
    return results
}

// ✅ ALSO GOOD: Pre-allocated length if you know exact size
func ProcessItems(n int) []Result {
    results := make([]Result, n)
    for i := 0; i < n; i++ {
        results[i] = process(i)
    }
    return results
}
```

### Slice Memory Leaks

```go
// ❌ BAD: Sub-slice keeps entire backing array in memory
func GetFirstTen(data []byte) []byte {
    // If data is 1GB, the entire 1GB stays in memory!
    return data[:10]
}

// ✅ GOOD: Copy to new slice
func GetFirstTen(data []byte) []byte {
    result := make([]byte, 10)
    copy(result, data[:10])
    return result
}
```

### Map Memory Management

```go
// Maps never shrink in Go - they only grow
type Cache struct {
    data map[string][]byte
}

// ❌ BAD: Map grows indefinitely
func (c *Cache) Set(key string, value []byte) {
    c.data[key] = value
}

func (c *Cache) Delete(key string) {
    delete(c.data, key)  // Memory not freed!
}

// ✅ GOOD: Recreate map periodically if size fluctuates
func (c *Cache) Cleanup() {
    if len(c.data) < cap(c.data)/2 {
        // Map is less than half full, recreate it
        newData := make(map[string][]byte, len(c.data))
        for k, v := range c.data {
            newData[k] = v
        }
        c.data = newData
    }
}
```

### Nil vs Empty Slices

```go
// ✅ GOOD: Prefer nil slices
var items []string  // nil slice, no allocation

// Only use empty slice if you need to distinguish in JSON
emptyItems := []string{}  // Empty slice, allocates
```

---

## 7. Interface Design

### Accept Interfaces, Return Structs

```go
// ✅ GOOD: Accept interface (flexible)
func SaveUser(w io.Writer, user *User) error {
    return json.NewEncoder(w).Encode(user)
}

// ✅ GOOD: Return concrete type (clear)
func NewUserService(db *sql.DB) *UserService {
    return &UserService{db: db}
}
```

### Define Interfaces at Usage Site

```go
// ❌ BAD: Interface defined with implementation
package database

type UserRepository interface {
    FindByID(id int) (*User, error)
    Save(user *User) error
}

type PostgresUserRepository struct{}

func (r *PostgresUserRepository) FindByID(id int) (*User, error) { }
func (r *PostgresUserRepository) Save(user *User) error { }

// ✅ GOOD: Interface defined where it's used
package service

// Only define what you need
type userRepository interface {
    FindByID(id int) (*User, error)
}

type UserService struct {
    repo userRepository
}

// Implementation in separate package
package database

type UserRepository struct{}

func (r *UserRepository) FindByID(id int) (*User, error) { }
func (r *UserRepository) Save(user *User) error { }
```
