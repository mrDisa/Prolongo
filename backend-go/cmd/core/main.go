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
	"github.com/mrDisa/Prolongo/backend-go/internal/config"
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

// run serves HTTP until ctx is cancelled, then lets running requests finish.
func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewRouter(logger),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: the SSE stream (G9) keeps responses open for hours.
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("core listening", slog.String("addr", cfg.HTTPAddr))
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
