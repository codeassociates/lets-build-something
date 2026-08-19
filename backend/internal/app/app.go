// Package app wires the components together. Both the API and the worker build
// the same object graph, so they always agree about rates, tax and reminders.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/codeassociates/lets-build-something/backend/internal/auth"
	"github.com/codeassociates/lets-build-something/backend/internal/billing"
	"github.com/codeassociates/lets-build-something/backend/internal/catalog"
	"github.com/codeassociates/lets-build-something/backend/internal/config"
	"github.com/codeassociates/lets-build-something/backend/internal/db"
	"github.com/codeassociates/lets-build-something/backend/internal/jobs"
	"github.com/codeassociates/lets-build-something/backend/internal/notify"
	"github.com/codeassociates/lets-build-something/backend/internal/rental"
)

type App struct {
	Config   *config.Config
	Pool     *db.DB
	Users    *auth.Store
	Catalog  *catalog.Store
	Billing  *billing.Store
	Queue    *jobs.Queue
	Rentals  *rental.Service
	Notifier *notify.Notifier
	Mailer   notify.Mailer
}

// Build connects to the database, migrates it, and constructs every store.
func Build(ctx context.Context, cfg *config.Config, migrate bool) (*App, error) {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if migrate {
		if err := db.Migrate(ctx, pool); err != nil {
			pool.Close()
			return nil, err
		}
	}

	users := auth.NewStore(pool)
	cat := catalog.NewStore(pool)

	// The payment gateway is an interface with one implementation today. A
	// Stripe adapter slots in here without touching anything downstream.
	gateway := billing.NewFakeGateway()
	bill := billing.NewStore(pool, gateway)

	queue := jobs.New(pool)
	rentals := rental.NewService(pool, cat, bill, queue, rental.Options{
		TaxRatePercent:     cfg.TaxRatePercent,
		LateFeeMultiple:    cfg.LateFeeMultiple,
		PickupReminderLead: cfg.PickupReminderLead,
		ReturnReminderLead: cfg.ReturnReminderLead,
	})

	var mailer notify.Mailer = &notify.SMTPMailer{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		From: cfg.MailFrom, FromName: cfg.MailName,
	}
	if cfg.SMTPHost == "" {
		slog.Warn("no SMTP host configured; emails will be recorded but not delivered")
		mailer = notify.NewMemoryMailer()
	}
	notifier := notify.New(pool, mailer, cfg.MailName, cfg.PublicBaseURL)

	return &App{
		Config: cfg, Pool: pool, Users: users, Catalog: cat, Billing: bill,
		Queue: queue, Rentals: rentals, Notifier: notifier, Mailer: mailer,
	}, nil
}

func (a *App) Close() {
	if a.Pool != nil {
		a.Pool.Close()
	}
}

// SetupLogging installs a structured logger. Text is easier to read while
// developing; JSON is what a log collector wants in a real deployment.
func SetupLogging() {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))
}

func LoadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}
	return cfg, nil
}
