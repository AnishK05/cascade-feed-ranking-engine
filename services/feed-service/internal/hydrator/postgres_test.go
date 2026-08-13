package hydrator

import (
	"context"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
)

func TestPostgresHydrateSkipsCacheAndCountsMisses(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	var gotIDs []int64
	h := &Postgres{load: func(_ context.Context, ids []int64) (map[int64]feed.Post, error) {
		gotIDs = append([]int64(nil), ids...)
		return map[int64]feed.Post{
			1: {ID: 1, AuthorID: 10, Content: "a", CreatedAt: now},
			2: {ID: 2, AuthorID: 20, Content: "b", CreatedAt: now},
		}, nil
	}}

	got, hits, misses, err := h.Hydrate(context.Background(), []int64{1, 2, 1})
	if err != nil {
		t.Fatalf("Hydrate() error = %v", err)
	}
	if hits != 0 || misses != 2 {
		t.Fatalf("hits=%d misses=%d, want 0 hits and 2 misses", hits, misses)
	}
	if len(gotIDs) != 2 || gotIDs[0] != 1 || gotIDs[1] != 2 {
		t.Fatalf("load IDs = %v, want unique [1 2]", gotIDs)
	}
	if got[1].Content != "a" || got[2].Content != "b" {
		t.Fatalf("posts = %+v", got)
	}
}

func TestPostgresHydrateEmpty(t *testing.T) {
	h := &Postgres{load: func(context.Context, []int64) (map[int64]feed.Post, error) {
		t.Fatal("load should not run for an empty ID list")
		return nil, nil
	}}
	got, hits, misses, err := h.Hydrate(context.Background(), nil)
	if err != nil || hits != 0 || misses != 0 || len(got) != 0 {
		t.Fatalf("Hydrate() = (%v, %d, %d, %v)", got, hits, misses, err)
	}
}
