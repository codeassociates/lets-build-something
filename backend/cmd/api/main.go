// Command api serves the REST API for the rental system.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codeassociates/lets-build-something/backend/internal/api"
	"github.com/codeassociates/lets-build-something/backend/internal/app"
	"github.com/codeassociates/lets-build-something/backend/internal/seed"
)

func main() {
	app.SetupLogging()
	if err := run(); err != nil {
		slog.Error("api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}

	// The API owns migrations: it is the one component guaranteed to be present.
	a, err := app.Build(ctx, cfg, true)
	if err != nil {
		return err
	}
	defer a.Close()

	// A fresh deployment can populate itself with demo data on first boot.
	if cfg.SeedOnBoot {
		s := seed.New(a.Pool, seed.Options{TaxRatePercent: cfg.TaxRatePercent})
		if err := s.Run(ctx, seed.Options{TaxRatePercent: cfg.TaxRatePercent}); err != nil {
			slog.Error("seeding failed", "err", err)
		}
	}

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: api.NewServer(api.Deps{
			Config: cfg, Pool: a.Pool, Users: a.Users, Catalog: a.Catalog,
			Rentals: a.Rentals, Billing: a.Billing, Notifier: a.Notifier, Queue: a.Queue,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api listening", "port", cfg.Port, "cors_origins", cfg.CORSOrigins)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
	}

	// Let in-flight requests finish before the process goes away.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
