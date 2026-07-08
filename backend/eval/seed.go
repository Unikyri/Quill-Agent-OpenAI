package eval

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/repositories"
	"github.com/quill/backend/internal/services"
	"github.com/quill/backend/internal/testutil"
)

const sagaUniverseIDString = "00000000-0000-0000-0000-000000000002"

// buildRealMemoryService constructs a MemoryService wired to real repositories
// against the saga universe. It applies migrations up to 019 so the saga seed
// data and consolidation table exist.
func buildRealMemoryService(t *testing.T, pool *pgxpool.Pool) (*services.MemoryService, uuid.UUID) {
	t.Helper()

	// testutil.RunMigrationsUpTo assumes the package is two levels below backend/
	// (../../migrations -> backend/migrations). eval lives at backend/eval, one
	// level below, so temporarily switch to a package that satisfies the path.
	runMigrationsForEval(t, pool)

	graphRepo := repositories.NewGraphRepo(pool)
	entityRepo := repositories.NewEntityRepo(pool)
	vectorRepo := repositories.NewVectorRepo(pool)
	consolidationRepo := repositories.NewConsolidationRepo(pool)

	svc := services.NewMemoryService(graphRepo, entityRepo, vectorRepo)
	svc.SetConsolidationRepo(consolidationRepo)

	return svc, uuid.MustParse(sagaUniverseIDString)
}

// runMigrationsForEval switches to backend/internal/services so that
// testutil.RunMigrationsUpTo's relative path resolves to backend/migrations.
func runMigrationsForEval(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir("../internal/services"); err != nil {
		t.Fatalf("chdir for migrations: %v", err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	testutil.RunMigrationsUpTo(t, pool, "019")
}

// makeSyntheticEmbedding returns a deterministic 1024-dim embedding.
//
// ponytail: used for fast harness smoke tests where real semantic similarity
// is not required; semantic evals should use Qwen embeddings instead.
func makeSyntheticEmbedding(index int) []float32 {
	vec := make([]float32, 1024)
	for i := range vec {
		vec[i] = float32((math.Sin(float64(index)*0.0174533*float64(i+1))+1.0)/2.0*0.9 + 0.05)
	}
	return vec
}

// resolveGoldIDs maps entity names in the gold corpus to their saga universe UUIDs.
func resolveGoldIDs(t *testing.T, pool *pgxpool.Pool, gold *GoldSet) {
	t.Helper()

	ctx := context.Background()
	universeID := uuid.MustParse(sagaUniverseIDString)
	entityRepo := repositories.NewEntityRepo(pool)

	resolve := func(name string) uuid.UUID {
		e, err := entityRepo.FindByName(ctx, universeID, name)
		if err != nil {
			t.Fatalf("resolve entity %q: %v", name, err)
		}
		return e.ID
	}

	for i := range gold.Queries {
		q := &gold.Queries[i]
		q.RelevantEntityIDs = nil
		for _, name := range q.RelevantEntityNames {
			q.RelevantEntityIDs = append(q.RelevantEntityIDs, resolve(name))
		}
	}

	gold.Forgetting.ShouldBeArchivedIDs = nil
	for _, name := range gold.Forgetting.ShouldBeArchived {
		gold.Forgetting.ShouldBeArchivedIDs = append(gold.Forgetting.ShouldBeArchivedIDs, resolve(name))
	}

	gold.Forgetting.MustStayActiveIDs = nil
	for _, name := range gold.Forgetting.MustStayActive {
		gold.Forgetting.MustStayActiveIDs = append(gold.Forgetting.MustStayActiveIDs, resolve(name))
	}
}
