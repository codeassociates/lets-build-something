package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
)

// Migrations are plain .sql files named NNNNN_name.sql, applied in filename
// order, each in its own transaction. A file may contain a "-- +down" marker;
// everything after it is the rollback and is stored but not executed here.
//
// This replaces a migration framework with about sixty lines because the needs
// are small: apply forward, once, safely, from several replicas at once.

const migrationLockID = 4711_2026

type Migration struct {
	Version  string
	Name     string
	UpSQL    string
	Checksum string
}

func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("reading migrations dir: %w", err)
	}

	var out []Migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		raw, err := fs.ReadFile(migrationsFS, "migrations/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", e.Name(), err)
		}
		body := string(raw)
		if i := strings.Index(body, "-- +down"); i >= 0 {
			body = body[:i]
		}
		version, name, _ := strings.Cut(strings.TrimSuffix(e.Name(), ".sql"), "_")
		sum := sha256.Sum256([]byte(body))
		out = append(out, Migration{
			Version:  version,
			Name:     name,
			UpSQL:    body,
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Migrate applies every migration not yet recorded. An advisory lock serialises
// concurrent starters, so several API replicas booting together is safe.
func Migrate(ctx context.Context, pool *DB) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("taking migration lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockID)

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			checksum    TEXT NOT NULL,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}

	applied := map[string]string{}
	rows, err := conn.Query(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("reading applied migrations: %w", err)
	}
	for rows.Next() {
		var v, c string
		if err := rows.Scan(&v, &c); err != nil {
			rows.Close()
			return err
		}
		applied[v] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range migrations {
		if have, ok := applied[m.Version]; ok {
			// An edited migration means the deployed schema and the source have
			// silently diverged. Say so rather than carrying on.
			if have != m.Checksum {
				return fmt.Errorf("migration %s_%s was modified after being applied "+
					"(checksum %s on disk, %s in database); add a new migration instead",
					m.Version, m.Name, m.Checksum[:8], have[:8])
			}
			continue
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning %s: %w", m.Version, err)
		}
		if _, err := tx.Exec(ctx, m.UpSQL); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("applying %s_%s: %w", m.Version, m.Name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`,
			m.Version, m.Name, m.Checksum); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("recording %s: %w", m.Version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing %s: %w", m.Version, err)
		}
		slog.Info("applied migration", "version", m.Version, "name", m.Name)
	}
	return nil
}
