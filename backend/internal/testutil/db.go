package testutil

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping test db: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func RunMigrationsUpTo(t *testing.T, pool *pgxpool.Pool, maxPrefix string) {
	t.Helper()

	ctx := context.Background()
	migrationsDir := filepath.Join("..", "..", "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = filepath.Join("..", "..", "..", "migrations")
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var upFiles, downFiles []string
	for _, e := range entries {
		f := filepath.Join(migrationsDir, e.Name())
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, f)
		} else if strings.HasSuffix(e.Name(), ".down.sql") {
			downFiles = append(downFiles, f)
		}
	}
	sort.Strings(upFiles)
	sort.Strings(downFiles)

	// Reset state by tearing down all known migrations in reverse order.
	for i := len(downFiles) - 1; i >= 0; i-- {
		sql, err := os.ReadFile(downFiles[i])
		if err != nil {
			t.Fatalf("read down migration %s: %v", downFiles[i], err)
		}
		// Ignore errors; tables may not exist yet.
		_, _ = pool.Exec(ctx, string(sql))
	}

	for _, f := range upFiles {
		if maxPrefix != "" && !strings.HasPrefix(filepath.Base(f), maxPrefix) && filepath.Base(f) > maxPrefix {
			continue
		}
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("execute migration %s: %v", f, err)
		}
	}
}

