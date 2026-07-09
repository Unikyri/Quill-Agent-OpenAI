package testutil

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse test db config: %v", err)
	}
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvector.RegisterTypes(ctx, conn)
	}
	// Cap MaxConns regardless of host core count or connection-string
	// overrides — concurrent test packages sharing one Postgres instance can
	// otherwise exceed max_connections on high-core CI/dev machines.
	poolCfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
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

// CheckAGE returns true if the Apache AGE extension is loaded (cypher function exists).
func CheckAGE(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM pg_proc WHERE proname = 'cypher')`,
	).Scan(&exists); err != nil {
		t.Fatalf("check for cypher function: %v", err)
	}
	return exists
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

	// Serialize the teardown+setup critical section across concurrent test
	// packages sharing the same TEST_DATABASE_URL. Session-level advisory locks
	// are bound to one physical connection, so a dedicated connection must be
	// used — a STANDALONE connection, not one checked out from the pool under
	// test: holding a pooled conn for the test's whole lifetime means
	// pool.Close() (called via t.Cleanup by SetupTestDB) blocks waiting for
	// this conn to be released, deadlocking against our own t.Cleanup which
	// only runs after the test's cleanups are processed in LIFO order — but
	// worse, other pool users can starve while it's held. A standalone conn
	// sidesteps the pool's lifecycle entirely.
	conn, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect dedicated connection for migration lock: %v", err)
	}

	lockCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if _, err := conn.Exec(lockCtx, `SELECT pg_advisory_lock(hashtext('quill_test_migrations'))`); err != nil {
		conn.Close(ctx)
		t.Fatalf("acquire migration advisory lock: %v", err)
	}

	// Hold the lock (and its dedicated connection) for the WHOLE test, not just
	// this teardown+setup critical section — released via t.Cleanup so it
	// survives until the test body (which shares schema state with whatever
	// ran here) has finished. Explicit unlock before Close is required:
	// a session-level lock is tied to the connection's backend session, so
	// unlocking before closing keeps the lock release observable/ordered
	// rather than relying on connection teardown to drop it implicitly.
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock(hashtext('quill_test_migrations'))`)
		conn.Close(ctx)
	})

	// Reset state by tearing down all known migrations in reverse order.
	// Down migrations silently ignore errors — the DB may not have the state yet.
	for i := len(downFiles) - 1; i >= 0; i-- {
		sql, err := os.ReadFile(downFiles[i])
		if err != nil {
			t.Fatalf("read down migration %s: %v", downFiles[i], err)
		}
		_, _ = conn.Exec(ctx, string(sql))
	}

	// ponytail: check AGE once before the up-migration loop
	ageAvailable := CheckAGE(t, pool)

	for _, f := range upFiles {
		if maxPrefix != "" && !strings.HasPrefix(filepath.Base(f), maxPrefix) && filepath.Base(f) > maxPrefix {
			continue
		}

		// Migration 014 requires Apache AGE (cypher function) for graph population.
		// Skip it when AGE is not available on the test DB.
		if !ageAvailable && strings.HasPrefix(filepath.Base(f), "014") {
			t.Log("skipping migration 014: Apache AGE extension not available (cypher function missing)")
			continue
		}

		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("execute migration %s: %v", f, err)
		}
	}
}

