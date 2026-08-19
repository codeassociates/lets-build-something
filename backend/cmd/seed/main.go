// Command seed fills the database with synthetic data for development and demos.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/codeassociates/lets-build-something/backend/internal/app"
	"github.com/codeassociates/lets-build-something/backend/internal/seed"
)

func main() {
	app.SetupLogging()

	reset := flag.Bool("reset", false,
		"delete all existing data before seeding (destructive)")
	randomSeed := flag.Int64("random-seed", 0,
		"seed for the random generator; the same value always produces the same yard")
	flag.Parse()

	if err := run(*reset, *randomSeed); err != nil {
		slog.Error("seeding failed", "err", err)
		os.Exit(1)
	}
}

func run(reset bool, randomSeed int64) error {
	ctx := context.Background()

	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}
	a, err := app.Build(ctx, cfg, true)
	if err != nil {
		return err
	}
	defer a.Close()

	opts := seed.Options{
		Reset:          reset,
		RandomSeed:     randomSeed,
		TaxRatePercent: cfg.TaxRatePercent,
	}
	if err := seed.New(a.Pool, opts).Run(ctx, opts); err != nil {
		return err
	}

	slog.Info("sign in with any seeded account", "password", seed.DemoPassword,
		"admin", "admin@kestrelrental.example",
		"staff", "marisol@kestrelrental.example",
		"customer", "dana.whitfield@example.com")
	return nil
}
