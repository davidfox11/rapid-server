package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rapidtrivia/rapid-server/internal/auth"
	"github.com/rapidtrivia/rapid-server/internal/platform"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	port := getenv("PORT", "8080")
	logLevel := getenv("LOG_LEVEL", "debug")
	otlpEndpoint := getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	serviceName := getenv("OTEL_SERVICE_NAME", "rapid-server")
	databaseURL := getenv("DATABASE_URL", "postgres://rapid:rapid@localhost:5432/rapid?sslmode=disable")
	redisURL := getenv("REDIS_URL", "redis://localhost:6379")
	authMode := getenv("AUTH_MODE", "dev")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(logLevel),
	}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 1. Observability
	otelShutdown, err := platform.InitObservability(ctx, serviceName, otlpEndpoint)
	if err != nil {
		slog.Error("init observability failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := otelShutdown(context.Background()); err != nil {
			slog.Error("observability shutdown error", "error", err)
		}
	}()

	// 2. Postgres
	pool, err := platform.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		slog.Error("init postgres failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 3. Redis
	rdb, err := platform.NewRedisClient(ctx, redisURL)
	if err != nil {
		slog.Error("init redis failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// 4. Auth
	var verifier auth.TokenVerifier
	switch authMode {
	case "firebase":
		fa, err := auth.NewFirebaseAuth(ctx)
		if err != nil {
			slog.Error("init firebase auth failed", "error", err)
			os.Exit(1)
		}
		verifier = fa
		slog.Info("auth mode: firebase")
	default:
		verifier = &auth.DevTokenVerifier{}
		slog.Info("auth mode: dev (accepts any token)")
	}

	authMiddleware := auth.Middleware(verifier)

	// 5. Routes
	mux := http.NewServeMux()

	// Public routes (no auth)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	// Protected routes (auth required)
	protected := http.NewServeMux()
	protected.HandleFunc("GET /auth/me", func(w http.ResponseWriter, r *http.Request) {
		uid, _ := auth.UserIDFromContext(r.Context())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"uid": uid})
	})
	mux.Handle("/", authMiddleware(protected))

	// 6. HTTP server
	handler := otelhttp.NewHandler(mux, "http",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: withTraceLogging(handler),
	}

	go func() {
		slog.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("server stopping")

	if err := srv.Shutdown(context.Background()); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("server stopped")
}

// withTraceLogging wraps a handler to add trace_id to log context for each request.
func withTraceLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := platform.TraceIDFromContext(r.Context())
		if traceID != "" {
			logger := slog.With("trace_id", traceID)
			ctx := context.WithValue(r.Context(), loggerKey{}, logger)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

type loggerKey struct{}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
