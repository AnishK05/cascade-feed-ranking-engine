package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresIntegrationCRUD(t *testing.T) {
	databaseURL := os.Getenv("POST_SERVICE_INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set POST_SERVICE_INTEGRATION_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	username := fmt.Sprintf("post_service_integration_%d", time.Now().UnixNano())
	var authorID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO public.users (username, display_name)
		VALUES ($1, 'Post Service Integration')
		RETURNING id`, username).Scan(&authorID); err != nil {
		t.Fatalf("insert test author: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM public.posts WHERE author_id = $1", authorID)
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM public.users WHERE id = $1", authorID)
	})

	repo := New(pool)
	created, err := repo.Create(ctx, authorID, "integration post", "https://example.com/media")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := repo.GetByIDs(ctx, []int64{created.ID, created.ID + 999999})
	if err != nil || got[created.ID].Content != created.Content {
		t.Fatalf("GetByIDs() = (%v, %v)", got, err)
	}
	if _, _, err := repo.Delete(ctx, created.ID, authorID+1); err != ErrForbidden {
		t.Fatalf("unauthorized Delete() error = %v, want ErrForbidden", err)
	}
	if _, _, err := repo.Delete(ctx, created.ID, authorID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	got, err = repo.GetByIDs(ctx, []int64{created.ID})
	if err != nil || len(got) != 0 {
		t.Fatalf("GetByIDs() after soft delete = (%v, %v)", got, err)
	}
}
