# Observability Patterns for Go

> Source: adapted from [nutcas3/best-practices](https://github.com/nutcas3/best-practices).

Structured logging, metrics, tracing, health checks, and service security patterns.

## Contents

- [Go Observability Patterns](#go-observability-patterns)
- [Security Patterns](#security-patterns)
- [Summary](#summary)
- [References](#references)

## Go Observability Patterns

### Structured Logging with Context

```go
// ✅ GOOD: Structured logging with context propagation
import (
    "context"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

type contextKey string

const loggerKey contextKey = "logger"

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
    return context.WithValue(ctx, loggerKey, logger)
}

func LoggerFromContext(ctx context.Context) *zap.Logger {
    if logger, ok := ctx.Value(loggerKey).(*zap.Logger); ok {
        return logger
    }
    return zap.L() // Default logger
}

func ProcessRequest(ctx context.Context, req Request) error {
    logger := LoggerFromContext(ctx).With(
        zap.String("request_id", GetRequestID(ctx)),
        zap.String("user_id", GetUserID(ctx)),
        zap.String("operation", "process_request"),
    )

    logger.Info("Starting request processing")
    
    // Process request...
    
    logger.Info("Completed request processing",
        zap.Duration("duration", time.Since(start)),
    )
    return nil
}
```

### Distributed Tracing

```go
// ✅ GOOD: OpenTelemetry tracing
import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func ProcessOrder(ctx context.Context, order Order) error {
    ctx, span := otel.Tracer("order-service").Start(ctx, "ProcessOrder")
    defer span.End()

    // Add span attributes
    span.SetAttributes(
        attribute.String("order.id", order.ID),
        attribute.String("order.customer", order.CustomerID),
        attribute.Float64("order.amount", order.Amount),
    )

    // Validate order
    if err := validateOrder(ctx, order); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "Order validation failed")
        return err
    }

    // Process payment
    if err := processPayment(ctx, order); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "Payment processing failed")
        return err
    }

    // Update inventory
    if err := updateInventory(ctx, order); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "Inventory update failed")
        return err
    }

    span.SetStatus(codes.Ok, "Order processed successfully")
    return nil
}

// ✅ GOOD: Tracing middleware
func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        
        // Start span
        tracer := otel.Tracer("http-server")
        ctx, span := tracer.Start(ctx, r.URL.Path,
            trace.WithAttributes(
                attribute.String("http.method", r.Method),
                attribute.String("http.url", r.URL.String()),
                attribute.String("http.user_agent", r.UserAgent()),
            ),
        )
        defer span.End()

        // Continue with traced context
        r = r.WithContext(ctx)
        next.ServeHTTP(w, r)

        // Record response status
        span.SetAttributes(
            attribute.Int("http.status_code", w.Header().Get("Status")),
        )
    })
}
```

### Metrics Collection

```go
// ✅ GOOD: Prometheus metrics
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    requestCount = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )

    requestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint"},
    )

    activeConnections = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "active_connections",
            Help: "Number of active connections",
        },
    )
)

func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Wrap response writer to capture status
        rw := &responseWriter{ResponseWriter: w}
        
        next.ServeHTTP(rw, r)
        
        // Record metrics
        duration := time.Since(start).Seconds()
        status := fmt.Sprintf("%d", rw.statusCode)
        
        requestCount.WithLabelValues(r.Method, r.URL.Path, status).Inc()
        requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}
```

### Health Checks

```go
// ✅ GOOD: Comprehensive health checks
type HealthChecker struct {
    checks map[string]CheckFunc
    mu     sync.RWMutex
}

type CheckFunc func(ctx context.Context) error

type HealthStatus struct {
    Status  string            `json:"status"`
    Checks  map[string]string `json:"checks"`
    Uptime  time.Duration     `json:"uptime"`
    Version string            `json:"version"`
}

func NewHealthChecker() *HealthChecker {
    return &HealthChecker{
        checks: make(map[string]CheckFunc),
    }
}

func (h *HealthChecker) AddCheck(name string, fn CheckFunc) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.checks[name] = fn
}

func (h *HealthChecker) Check(ctx context.Context) HealthStatus {
    h.mu.RLock()
    defer h.mu.RUnlock()

    status := HealthStatus{
        Checks:  make(map[string]string),
        Uptime:  time.Since(startTime),
        Version: version,
    }

    allHealthy := true
    for name, check := range h.checks {
        checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()
        
        if err := check(checkCtx); err != nil {
            status.Checks[name] = "unhealthy: " + err.Error()
            allHealthy = false
        } else {
            status.Checks[name] = "healthy"
        }
    }

    if allHealthy {
        status.Status = "healthy"
    } else {
        status.Status = "unhealthy"
    }

    return status
}

// Example health checks
func (s *Service) DatabaseHealthCheck(ctx context.Context) error {
    if err := s.db.PingContext(ctx); err != nil {
        return fmt.Errorf("database ping failed: %w", err)
    }
    return nil
}

func (s *Service) RedisHealthCheck(ctx context.Context) error {
    if err := s.redis.Ping(ctx).Err(); err != nil {
        return fmt.Errorf("redis ping failed: %w", err)
    }
    return nil
}
```

## Security Patterns

### Go Security Patterns

```go
// ✅ GOOD: Rate limiting with token bucket
type RateLimiter struct {
    tokens     map[string]*tokenBucket
    mu         sync.RWMutex
    refillRate time.Duration
    capacity   int
}

type tokenBucket struct {
    tokens    int
    lastRefill time.Time
    mu        sync.Mutex
}

func (rl *RateLimiter) Allow(key string) bool {
    rl.mu.RLock()
    bucket, exists := rl.tokens[key]
    if !exists {
        rl.mu.RUnlock()
        rl.mu.Lock()
        bucket = &tokenBucket{
            tokens:    rl.capacity,
            lastRefill: time.Now(),
        }
        rl.tokens[key] = bucket
        rl.mu.Unlock()
    } else {
        rl.mu.RUnlock()
    }

    bucket.mu.Lock()
    defer bucket.mu.Unlock()

    // Refill tokens
    now := time.Now()
    elapsed := now.Sub(bucket.lastRefill)
    tokensToAdd := int(elapsed / rl.refillRate)
    if tokensToAdd > 0 {
        bucket.tokens = min(bucket.tokens+tokensToAdd, rl.capacity)
        bucket.lastRefill = now
    }

    if bucket.tokens > 0 {
        bucket.tokens--
        return true
    }

    return false
}

// ✅ GOOD: Input validation and sanitization
type Validator struct {
    sanitizer *bluemonday.Policy
}

func NewValidator() *Validator {
    p := bluemonday.StrictPolicy()
    p.AllowElements("b", "i", "em", "strong")
    
    return &Validator{
        sanitizer: p,
    }
}

func (v *Validator) ValidateAndSanitize(input string) (string, error) {
    // Check length
    if len(input) > 1000 {
        return "", errors.New("input too long")
    }

    // Sanitize HTML
    sanitized := v.sanitizer.Sanitize(input)

    // Additional validation
    if strings.TrimSpace(sanitized) == "" {
        return "", errors.New("input cannot be empty")
    }

    return sanitized, nil
}

// ✅ GOOD: Secure headers middleware
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        
        next.ServeHTTP(w, r)
    })
}
```

## Summary

These observability and security patterns provide:

1. **Observability**: Structured logging, distributed tracing, metrics, health checks
2. **Security**: Rate limiting, input validation, secure headers, authentication
3. **Integration**: Seamless integration with existing monitoring systems
4. **Performance**: Low-overhead instrumentation
5. **Standards**: OpenTelemetry, Prometheus, Sentry integration

Remember to:
- Instrument early in development
- Use structured data for better querying
- Set appropriate sampling rates for tracing
- Monitor the monitoring system itself
- Keep security configurations up to date

## References

1. **OpenTelemetry**: https://opentelemetry.io/
2. **Prometheus**: https://prometheus.io/
3. **Tracing in Go**: https://opentelemetry.io/docs/instrumentation/go/
