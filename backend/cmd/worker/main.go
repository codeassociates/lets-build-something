// Command worker sends the scheduled emails and runs the daily sweep.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/codeassociates/lets-build-something/backend/internal/app"
	"github.com/codeassociates/lets-build-something/backend/internal/worker"
)

func main() {
	app.SetupLogging()
	if err := run(); err != nil {
		slog.Error("worker exited", "err", err)
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

	// The API applies migrations; the worker waits for the schema to exist
	// rather than racing to create it.
	a, err := app.Build(ctx, cfg, false)
	if err != nil {
		return err
	}
	defer a.Close()

	w := worker.New(a.Queue, a.Rentals, a.Billing, a.Notifier, a.Users, worker.Options{
		PollInterval:    cfg.WorkerPollInterval,
		LateFeeMultiple: cfg.LateFeeMultiple,
	})
	slog.Info("worker configured", "mailer", a.Mailer.Name())
	return w.Run(ctx)
}
