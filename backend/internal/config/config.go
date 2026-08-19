// Package config loads process configuration from the environment.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port          int
	DatabaseURL   string
	SessionTTL    time.Duration
	CORSOrigins   []string
	PublicBaseURL string

	SMTPHost  string
	SMTPPort  int
	MailFrom  string
	MailName  string
	MailDebug bool

	TaxRatePercent  float64
	LateFeeMultiple float64

	// How far ahead of a pickup or return the reminder job fires.
	PickupReminderLead time.Duration
	ReturnReminderLead time.Duration
	WorkerPollInterval time.Duration
	SeedOnBoot         bool
}

func Load() (*Config, error) {
	c := &Config{
		Port:               envInt("PORT", 8080),
		SessionTTL:         time.Duration(envInt("SESSION_TTL_HOURS", 24*14)) * time.Hour,
		PublicBaseURL:      env("PUBLIC_BASE_URL", "http://localhost:5173"),
		SMTPHost:           env("SMTP_HOST", "mailpit"),
		SMTPPort:           envInt("SMTP_PORT", 1025),
		MailFrom:           env("MAIL_FROM", "rentals@example.com"),
		MailName:           env("MAIL_FROM_NAME", "Kestrel Equipment Rental"),
		MailDebug:          envBool("MAIL_DEBUG", false),
		TaxRatePercent:     envFloat("TAX_RATE_PERCENT", 8.5),
		LateFeeMultiple:    envFloat("LATE_FEE_MULTIPLE", 1.5),
		PickupReminderLead: time.Duration(envInt("PICKUP_REMINDER_LEAD_HOURS", 24)) * time.Hour,
		ReturnReminderLead: time.Duration(envInt("RETURN_REMINDER_LEAD_HOURS", 24)) * time.Hour,
		WorkerPollInterval: time.Duration(envInt("WORKER_POLL_SECONDS", 15)) * time.Second,
		SeedOnBoot:         envBool("SEED_ON_BOOT", false),
	}

	c.DatabaseURL = buildDatabaseURL()
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL or DATABASE_HOST must be set")
	}
	c.CORSOrigins = splitList(env("CORS_ORIGINS", "http://localhost:5173"))
	return c, nil
}

// buildDatabaseURL prefers an explicit DATABASE_URL, but assembles one from parts
// otherwise. stack delivers POSTGRES_PASSWORD as a generated secret in the
// environment, so it must never be baked into a connection string in a compose file.
func buildDatabaseURL() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	host := env("DATABASE_HOST", "")
	if host == "" {
		return ""
	}
	user := env("DATABASE_USER", "postgres")
	pass := env("POSTGRES_PASSWORD", env("DATABASE_PASSWORD", ""))
	name := env("DATABASE_NAME", "rentals")
	port := envInt("DATABASE_PORT", 5432)
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, pass),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Path:     "/" + name,
		RawQuery: "sslmode=" + env("DATABASE_SSLMODE", "disable"),
	}).String()
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func envFloat(k string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		if r != ' ' {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
