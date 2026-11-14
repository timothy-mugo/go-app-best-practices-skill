# Go Testing Patterns

> Source: adapted from [nutcas3/best-practices](https://github.com/nutcas3/best-practices).

Table tests, mocks, integration, benchmarks, fuzzing.

## Contents

- [Go Testing Patterns](#go-testing-patterns)
- [Summary](#summary)
- [References](#references)

## Go Testing Patterns

### Property-Based Testing

```go
// ✅ GOOD: Property-based tests with testify
import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"
)

func TestAdditionProperties(t *testing.T) {
    t.Run("commutative", func(t *testing.T) {
        rapid.Check(t, func(t *rapid.T) {
            a := rapid.Int().Draw(t, "a")
            b := rapid.Int().Draw(t, "b")
            
            result1 := add(a, b)
            result2 := add(b, a)
            
            assert.Equal(t, result1, result2)
        })
    })

    t.Run("associative", func(t *testing.T) {
        rapid.Check(t, func(t *rapid.T) {
            a := rapid.Int().Draw(t, "a")
            b := rapid.Int().Draw(t, "b")
            c := rapid.Int().Draw(t, "c")
            
            result1 := add(add(a, b), c)
            result2 := add(a, add(b, c))
            
            assert.Equal(t, result1, result2)
        })
    })

    t.Run("identity", func(t *testing.T) {
        rapid.Check(t, func(t *rapid.T) {
            x := rapid.Int().Draw(t, "x")
            
            result1 := add(x, 0)
            result2 := add(0, x)
            
            assert.Equal(t, x, result1)
            assert.Equal(t, x, result2)
        })
    })
}
```

### Contract Testing

```go
// ✅ GOOD: Contract tests for API contracts
import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/stretchr/testify/assert"
)

type ContractTest struct {
    name           string
    request        Request
    expectedStatus int
    expectedBody   interface{}
    setup          func(*testing.T) *Service
}

func TestAPIContracts(t *testing.T) {
    tests := []ContractTest{
        {
            name: "create user with valid data",
            request: Request{
                Method: "POST",
                Path:   "/api/users",
                Body: User{
                    Name:  "John Doe",
                    Email: "john@example.com",
                },
            },
            expectedStatus: http.StatusCreated,
            expectedBody: User{
                ID:    "generated-id",
                Name:  "John Doe",
                Email: "john@example.com",
            },
            setup: func(t *testing.T) *Service {
                return NewTestService(t)
            },
        },
        {
            name: "create user with invalid email",
            request: Request{
                Method: "POST",
                Path:   "/api/users",
                Body: User{
                    Name:  "John Doe",
                    Email: "invalid-email",
                },
            },
            expectedStatus: http.StatusBadRequest,
            expectedBody: ErrorResponse{
                Code:    "INVALID_INPUT",
                Message: "Invalid email format",
            },
            setup: func(t *testing.T) *Service {
                return NewTestService(t)
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service := tt.setup(t)
            handler := NewHandler(service)

            body, _ := json.Marshal(tt.request.Body)
            req := httptest.NewRequest(tt.request.Method, tt.request.Path, bytes.NewBuffer(body))
            w := httptest.NewRecorder()

            handler.ServeHTTP(w, req)

            assert.Equal(t, tt.expectedStatus, w.Code)
            
            var actual interface{}
            json.Unmarshal(w.Body.Bytes(), &actual)
            assert.Equal(t, tt.expectedBody, actual)
        })
    }
}
```

### Integration Testing with Test Containers

```go
// ✅ GOOD: Integration tests with test containers
import (
    "context"
    "database/sql"
    "testing"
    "time"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    _ "github.com/lib/pq"
)

func TestUserRepositoryIntegration(t *testing.T) {
    // Start PostgreSQL container
    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        Image:        "postgres:15",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_DB":       "testdb",
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
        },
        WaitingFor: wait.ForLog("database system is ready to accept connections"),
    }

    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    defer container.Terminate(ctx)

    // Get connection details
    host, err := container.Host(ctx)
    require.NoError(t, err)

    port, err := container.MappedPort(ctx, "5432")
    require.NoError(t, err)

    // Connect to database
    dbURL := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())
    db, err := sql.Open("postgres", dbURL)
    require.NoError(t, err)
    defer db.Close()

    // Wait for database to be ready
    require.Eventually(t, func() bool {
        return db.Ping() == nil
    }, 30*time.Second, time.Second)

    // Run migrations
    err = runMigrations(db)
    require.NoError(t, err)

    // Test repository
    repo := NewUserRepository(db)
    
    user := &User{
        Name:  "Test User",
        Email: "test@example.com",
    }
    
    // Create user
    err = repo.Create(user)
    require.NoError(t, err)
    assert.NotEmpty(t, user.ID)

    // Get user
    found, err := repo.GetByID(user.ID)
    require.NoError(t, err)
    assert.Equal(t, user.Name, found.Name)
    assert.Equal(t, user.Email, found.Email)

    // Update user
    user.Name = "Updated Name"
    err = repo.Update(user)
    require.NoError(t, err)

    // Verify update
    found, err = repo.GetByID(user.ID)
    require.NoError(t, err)
    assert.Equal(t, "Updated Name", found.Name)

    // Delete user
    err = repo.Delete(user.ID)
    require.NoError(t, err)

    // Verify deletion
    found, err = repo.GetByID(user.ID)
    assert.Error(t, err)
    assert.Nil(t, found)
}
```

### Load Testing

```go
// ✅ GOOD: Load testing with vegeta
import (
    "fmt"
    "net/http"
    "testing"
    "time"
    "github.com/tsenart/vegeta/v12/lib"
)

func TestAPIPerformance(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test in short mode")
    }

    rate := vegeta.Rate{Freq: 100, Per: time.Second}
    duration := 30 * time.Second
    targets := []vegeta.Target{
        {
            Method: "GET",
            URL:    "http://localhost:8080/api/users",
            Header: http.Header{
                "Content-Type": []string{"application/json"},
            },
        },
        {
            Method: "POST",
            URL:    "http://localhost:8080/api/users",
            Body:   []byte(`{"name":"Test User","email":"test@example.com"}`),
            Header: http.Header{
                "Content-Type": []string{"application/json"},
            },
        },
    }

    attacker := vegeta.NewAttacker()
    var metrics vegeta.Metrics

    for _, target := range targets {
        results := attacker.Attack(vegeta.NewStaticTargeter(target), rate, duration, fmt.Sprintf("Load test %s", target.URL))
        for res := range results {
            metrics.Add(res)
        }
    }

    metrics.Close()

    // Assert performance requirements
    assert.Less(t, metrics.Latencies.P95, 100*time.Millisecond, "95th percentile latency should be < 100ms")
    assert.Less(t, metrics.Latencies.P99, 500*time.Millisecond, "99th percentile latency should be < 500ms")
    assert.Greater(t, metrics.Success, 0.95, "Success rate should be > 95%")
    assert.Less(t, metrics.Errors, 0.05, "Error rate should be < 5%")

    // Print detailed metrics
    t.Logf("Performance metrics:")
    t.Logf("  Requests: %d", metrics.Requests)
    t.Logf("  Success rate: %.2f%%", metrics.Success*100)
    t.Logf("  Latency P50: %v", metrics.Latencies.P50)
    t.Logf("  Latency P95: %v", metrics.Latencies.P95)
    t.Logf("  Latency P99: %v", metrics.Latencies.P99)
    t.Logf("  Throughput: %.2f req/s", metrics.Throughput)
}
```

### Chaos Testing

```go
// ✅ GOOD: Chaos testing for resilience
import (
    "context"
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

type ChaosInjector interface {
    InjectLatency(ctx context.Context, duration time.Duration) error
    InjectError(ctx context.Context, errorRate float64) error
    Stop() error
}

func TestServiceResilience(t *testing.T) {
    tests := []struct {
        name           string
        chaos          func(ctx context.Context, injector ChaosInjector)
        expectedResult string
        shouldSucceed  bool
    }{
        {
            name: "normal operation",
            chaos: func(ctx context.Context, injector ChaosInjector) {
                // No chaos injection
            },
            expectedResult: "success",
            shouldSucceed:  true,
        },
        {
            name: "database latency",
            chaos: func(ctx context.Context, injector ChaosInjector) {
                injector.InjectLatency(ctx, 2*time.Second)
            },
            expectedResult: "success",
            shouldSucceed:  true,
        },
        {
            name: "database errors",
            chaos: func(ctx context.Context, injector ChaosInjector) {
                injector.InjectError(ctx, 0.3) // 30% error rate
            },
            expectedResult: "success",
            shouldSucceed:  true,
        },
        {
            name: "high latency and errors",
            chaos: func(ctx context.Context, injector ChaosInjector) {
                injector.InjectLatency(ctx, 5*time.Second)
                injector.InjectError(ctx, 0.5) // 50% error rate
            },
            expectedResult: "error",
            shouldSucceed:  false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup chaos injector
            injector := NewChaosInjector(t)
            defer injector.Stop()

            // Setup service with circuit breaker and retries
            service := NewServiceWithResilience(injector)

            // Inject chaos
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            defer cancel()

            tt.chaos(ctx, injector)

            // Test service
            result, err := service.ProcessRequest(ctx, "test-request")

            if tt.shouldSucceed {
                require.NoError(t, err)
                assert.Equal(t, tt.expectedResult, result)
            } else {
                assert.Error(t, err)
            }
        })
    }
}
```

## Summary

These testing patterns provide:

1. **Property-Based Testing**: Verify invariants across many inputs
2. **Contract Testing**: Ensure API contracts are maintained
3. **Integration Testing**: Test real components with test containers
4. **Load Testing**: Verify performance under load
5. **Chaos Testing**: Test system resilience
6. **Mock Testing**: Isolate components for unit testing
7. **Fuzz Testing**: Find edge cases and security issues

Remember to:
- Test both happy paths and error cases
- Use realistic test data
- Test performance characteristics
- Include security testing
- Automate testing in CI/CD
- Monitor test coverage
- Keep tests maintainable

## References

1. **Go Testing**: https://golang.org/pkg/testing/
2. **Testify**: https://github.com/stretchr/testify
3. **Proptest**: https://github.com/BurntSushi/ripgrep
4. **Mockall**: https://github.com/asomers/mockall
5. **Criterion**: https://github.com/bheisler/criterion.rs
