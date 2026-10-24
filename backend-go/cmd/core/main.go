// Command core is the Prolongo risk engine and panel API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/api"
	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
	"github.com/mrDisa/Prolongo/backend-go/internal/config"
	"github.com/mrDisa/Prolongo/backend-go/internal/db"
	"github.com/mrDisa/Prolongo/backend-go/internal/settings"
	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

const shutdownTimeout = 10 * time.Second

func main() {
	healthcheck := flag.Bool("healthcheck", false, "GET /healthz on the local server and exit 0 if it answers 200")
	flag.Parse()

	if *healthcheck {
		os.Exit(runHealthcheck(os.Getenv("HTTP_ADDR")))
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	level, _ := cfg.SlogLevel() // already validated by config.Load
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("core stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}

// Startup timings from docs/ТЗ_BACKEND_GO.md §5 and §13.
const (
	dbConnectTimeout  = 60 * time.Second
	sourceWaitTimeout = 60 * time.Second
	sessionCleanup    = time.Hour
)

// run prepares the database and serves HTTP until ctx is cancelled.
// Order: PostgreSQL → Django's source schema → core migrations → source data → model settings → admin user → HTTP.
func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	connectCtx, cancel := context.WithTimeout(ctx, dbConnectTimeout)
	pool, err := db.Open(connectCtx, cfg.DatabaseURL, logger)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.WaitForSource(ctx, pool, sourceWaitTimeout, logger); err != nil {
		return err
	}
	if err := db.Migrate(ctx, pool, logger); err != nil {
		return err
	}

	st := store.New(pool)
	// Smoke check that core sees ingest's data. G7 replaces this with the recalculation worker.
	ds, err := st.LoadDataset(ctx)
	if err != nil {
		return err
	}
	accounts, services, transactions := ds.Counts()
	logger.Info("source data loaded",
		slog.Int("accounts", accounts), slog.Int("services", services), slog.Int("transactions", transactions),
		slog.Time("latest_timestamp", ds.LatestTimestamp), slog.Any("source_time", ds.SourceTime))

	settingsSvc := settings.NewService(st)
	current, err := settingsSvc.Ensure(ctx)
	if err != nil {
		return err
	}
	logger.Info("model settings ready", slog.Int64("version", current.Version), slog.Int("horizon_days", current.HorizonDays))

	authSvc, err := auth.NewService(st, time.Duration(cfg.SessionTTLHours)*time.Hour)
	if err != nil {
		return err
	}
	if err := authSvc.EnsureAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return err
	}
	logger.Info("admin user ready", slog.String("username", cfg.AdminUsername))
	go authSvc.CleanupLoop(ctx, sessionCleanup, logger)

	handler := api.NewRouter(api.Deps{
		Logger:      logger,
		Auth:        authSvc,
		Settings:    settingsSvc,
		Cookie:      api.CookieConfig{Name: cfg.SessionCookieName, Secure: cfg.CookieSecure},
		OIDCEnabled: cfg.OIDC.Enabled,
	})
	return serve(ctx, cfg.HTTPAddr, handler, logger)
}

// serve runs the HTTP server until ctx is cancelled, then lets running requests finish.
func serve(ctx context.Context, addr string, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: the SSE stream (G9) keeps responses open for hours.
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("core listening", slog.String("addr", addr))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down", slog.String("timeout", shutdownTimeout.String()))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("core stopped")
	return nil
}

// runHealthcheck is used by the compose healthcheck: distroless has no wget or curl.
func runHealthcheck(addr string) int {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(healthURL(addr))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}

// healthURL turns HTTP_ADDR (":8000", "0.0.0.0:8000", "") into a loopback URL.
func healthURL(addr string) string {
	if addr == "" {
		addr = ":8000"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "8000"
	}
	return "http://" + net.JoinHostPort("127.0.0.1", port) + "/healthz"
}
