package repositories

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quill/backend/internal/models"
	"github.com/quill/backend/internal/testutil"
)

func createTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *models.User {
	t.Helper()
	user := &models.User{ID: uuid.New(), Email: uuid.NewString() + "@test.local", DisplayName: "Test"}
	if _, err := pool.Exec(ctx,
		"INSERT INTO users (id, email, password_hash, display_name) VALUES ($1, $2, $3, $4)",
		user.ID, user.Email, "hash", user.DisplayName); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestUniverseRepoCRUD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "004")
	ctx := context.Background()

	user := createTestUser(t, ctx, pool)
	repo := NewUniverseRepo(pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	u := &models.Universe{
		ID:     uuid.New(),
		UserID: user.ID,
		Name:   "Test Universe",
		Format: "novel",
	}
	if err := repo.Create(ctx, tx, u); err != nil {
		t.Fatalf("create universe: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	found, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("find universe: %v", err)
	}
	if found.Name != u.Name {
		t.Errorf("name = %q, want %q", found.Name, u.Name)
	}

	list, total, err := repo.ListByUser(ctx, user.ID, 1, 10)
	if err != nil {
		t.Fatalf("list universes: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("list total=%d len=%d, want 1,1", total, len(list))
	}

	updateTx, _ := pool.Begin(ctx)
	u.Name = "Updated Universe"
	if err := repo.Update(ctx, updateTx, u); err != nil {
		t.Fatalf("update universe: %v", err)
	}
	updateTx.Commit(ctx)

	delTx, _ := pool.Begin(ctx)
	if err := repo.Delete(ctx, delTx, u.ID); err != nil {
		t.Fatalf("delete universe: %v", err)
	}
	delTx.Commit(ctx)

	_, err = repo.FindByID(ctx, u.ID)
	if err == nil {
		t.Error("expected error after deleting universe")
	}
}

func TestWorkRepoCRUD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "004")
	ctx := context.Background()

	user := createTestUser(t, ctx, pool)
	universeRepo := NewUniverseRepo(pool)
	workRepo := NewWorkRepo(pool)

	utx, _ := pool.Begin(ctx)
	u := &models.Universe{ID: uuid.New(), UserID: user.ID, Name: "U", Format: "novel"}
	if err := universeRepo.Create(ctx, utx, u); err != nil {
		t.Fatalf("create universe: %v", err)
	}
	utx.Commit(ctx)

	wtx, _ := pool.Begin(ctx)
	w := &models.Work{
		ID:         uuid.New(),
		UniverseID: u.ID,
		Title:      "Test Work",
		Type:       "book",
		OrderIndex: 1,
		Status:     "in_progress",
	}
	if err := workRepo.Create(ctx, wtx, w); err != nil {
		t.Fatalf("create work: %v", err)
	}
	wtx.Commit(ctx)

	found, err := workRepo.FindByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("find work: %v", err)
	}
	if found.Title != w.Title {
		t.Errorf("title = %q, want %q", found.Title, w.Title)
	}

	list, err := workRepo.ListByUniverse(ctx, u.ID)
	if err != nil {
		t.Fatalf("list works: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len works = %d, want 1", len(list))
	}

	maxOrder, err := workRepo.GetMaxOrderIndex(ctx, u.ID)
	if err != nil {
		t.Fatalf("max order: %v", err)
	}
	if maxOrder != 1 {
		t.Errorf("max order = %d, want 1", maxOrder)
	}
}

func TestChapterRepoCRUD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.RunMigrationsUpTo(t, pool, "004")
	ctx := context.Background()

	user := createTestUser(t, ctx, pool)
	universeRepo := NewUniverseRepo(pool)
	workRepo := NewWorkRepo(pool)
	chapterRepo := NewChapterRepo(pool)

	utx, _ := pool.Begin(ctx)
	u := &models.Universe{ID: uuid.New(), UserID: user.ID, Name: "U", Format: "novel"}
	universeRepo.Create(ctx, utx, u)
	utx.Commit(ctx)

	wtx, _ := pool.Begin(ctx)
	w := &models.Work{ID: uuid.New(), UniverseID: u.ID, Title: "W", Type: "book", OrderIndex: 1, Status: "in_progress"}
	workRepo.Create(ctx, wtx, w)
	wtx.Commit(ctx)

	ctxTx, _ := pool.Begin(ctx)
	c := &models.Chapter{
		ID:         uuid.New(),
		WorkID:     w.ID,
		Title:      "Test Chapter",
		OrderIndex: 1,
		Content:    "chapter content",
		RawText:    "raw text",
		WordCount:  2,
		Status:     "draft",
	}
	if err := chapterRepo.Create(ctx, ctxTx, c); err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	ctxTx.Commit(ctx)

	found, err := chapterRepo.FindByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("find chapter: %v", err)
	}
	if found.Title != c.Title {
		t.Errorf("title = %q, want %q", found.Title, c.Title)
	}

	list, err := chapterRepo.ListByWork(ctx, w.ID)
	if err != nil {
		t.Fatalf("list chapters: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len chapters = %d, want 1", len(list))
	}

	if got := chapterRepo.CountWords("one two three"); got != 3 {
		t.Errorf("CountWords = %d, want 3", got)
	}
}

// compile-time guard: ensure pgx.Tx is accepted
var _ pgx.Tx = (pgx.Tx)(nil)
