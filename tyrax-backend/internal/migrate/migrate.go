package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrateTimeout = 30 * time.Second

// Apply runs pending SQL files from dir against the database.
// Applied files are tracked in schema_migrations (filename without path).
//
// Existing databases created by Postgres initdb.d (001..014) are bootstrapped
// automatically so only missing migrations (e.g. 015) are executed.
func Apply(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	ctx, cancel := context.WithTimeout(ctx, migrateTimeout)
	defer cancel()

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	if err := bootstrapLegacy(ctx, pool, files); err != nil {
		return err
	}

	for _, name := range files {
		var exists bool
		if err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)", name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists {
			continue
		}

		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (filename) VALUES ($1)", name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

// bootstrapLegacy marks migrations already applied by Postgres initdb.d on
// existing deployments, so Apply only runs what's actually missing.
func bootstrapLegacy(ctx context.Context, pool *pgxpool.Pool, files []string) error {
	var tracked int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&tracked); err != nil {
		return fmt.Errorf("count schema_migrations: %w", err)
	}
	if tracked > 0 {
		return nil
	}

	var usersExist bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			 WHERE table_schema = 'public' AND table_name = 'users'
		)`).Scan(&usersExist); err != nil {
		return fmt.Errorf("check users table: %w", err)
	}
	if !usersExist {
		return nil
	}

	for _, name := range files {
		if pending, err := migrationStillNeeded(ctx, pool, name); err != nil {
			return err
		} else if pending {
			continue
		}
		if _, err := pool.Exec(ctx,
			"INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING", name); err != nil {
			return fmt.Errorf("bootstrap migration %s: %w", name, err)
		}
	}
	return nil
}

func migrationStillNeeded(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	switch name {
	case "015_admin_support.sql":
		var colExists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				 WHERE table_schema = 'public'
				   AND table_name = 'users'
				   AND column_name = 'registration_ip'
			)`).Scan(&colExists)
		if err != nil {
			return false, fmt.Errorf("check registration_ip column: %w", err)
		}
		return !colExists, nil
	case "016_email_verification.sql":
		var tableExists bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				 WHERE table_schema = 'public' AND table_name = 'email_verifications'
			)`).Scan(&tableExists)
		if err != nil {
			return false, fmt.Errorf("check email_verifications table: %w", err)
		}
		return !tableExists, nil
	default:
		return false, nil
	}
}
