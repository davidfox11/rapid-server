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
	"github.com/rapidtrivia/rapid-server/internal/db"
	"github.com/rapidtrivia/rapid-server/internal/friend"
	"github.com/rapidtrivia/rapid-server/internal/game"
	"github.com/rapidtrivia/rapid-server/internal/platform"
	"github.com/rapidtrivia/rapid-server/internal/user"
	"github.com/rapidtrivia/rapid-server/internal/ws"
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

	pool, err := platform.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		slog.Error("init postgres failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := platform.NewRedisClient(ctx, redisURL)
	if err != nil {
		slog.Error("init redis failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

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

	queries := db.New(pool)
	userStore := user.NewStore(queries)
	friendStore := friend.NewStore(queries)
	gameStore := game.NewStore(queries, pool)

	hub := ws.NewHub(logger, userStore, friendStore)
	go hub.Run(ctx)

	gameStarter := game.NewStarter(gameStore, hub, logger)
	hub.SetGameStarter(gameStarter)

	userHandler := user.NewHandler(userStore, logger)
	friendHandler := friend.NewHandler(friendStore, userStore, hub, logger)
	gameHandler := game.NewHandler(gameStore, userStore, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	protected := http.NewServeMux()
	protected.Handle("POST /auth/register", userHandler.Register())
	protected.Handle("GET /auth/me", userHandler.Me())
	protected.Handle("POST /friends/request", friendHandler.Request())
	protected.Handle("POST /friends/respond", friendHandler.Respond())
	protected.Handle("GET /friends", friendHandler.List())
	protected.Handle("GET /friends/search", friendHandler.Search())
	protected.Handle("GET /categories", gameHandler.ListCategories())
	protected.Handle("GET /matches", gameHandler.ListMatches())
	protected.Handle("GET /matches/{id}", gameHandler.GetMatch())
	protected.HandleFunc("GET /ws", hub.HandleWebSocket)
	mux.Handle("/", authMiddleware(protected))

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
