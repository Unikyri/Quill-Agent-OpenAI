package testutil

import (
	"os"
	"strings"
	"testing"
)

// TestSetupTestDBCapsMaxConns verifies SetupTestDB caps the pool's MaxConns
// at 8 regardless of host core count or connection-string overrides,
// preventing concurrent test packages from exceeding Postgres
// max_connections on high-core CI/dev machines.
//
// A plain "does the default exceed 8" check is host-dependent (pgxpool's
// default is max(4, NumCPU), which happens to equal 8 on an 8-core box), so
// this forces the scenario deterministically: append pool_max_conns=100 to
// the connection string (which pgxpool.ParseConfig would otherwise honor)
// and assert SetupTestDB still caps it at 8.
func TestSetupTestDBCapsMaxConns(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	t.Setenv("TEST_DATABASE_URL", base+sep+"pool_max_conns=100")

	pool := SetupTestDB(t)
	if got := pool.Config().MaxConns; got > 8 {
		t.Fatalf("MaxConns=%d, want <=8 (connection string requested 100)", got)
	}
}
