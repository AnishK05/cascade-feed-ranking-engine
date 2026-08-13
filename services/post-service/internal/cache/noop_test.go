package cache

import (
	"context"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
)

func TestNoopNeverReturnsCachedPosts(t *testing.T) {
	c := NewNoop()
	ctx := context.Background()
	p := post.Post{ID: 7, AuthorID: 1, Content: "x", CreatedAt: time.Now()}
	if err := c.Set(ctx, p); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := c.SetMany(ctx, []post.Post{p}); err != nil {
		t.Fatalf("SetMany() error = %v", err)
	}
	got, err := c.GetMany(ctx, []int64{7})
	if err != nil {
		t.Fatalf("GetMany() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("GetMany() = %+v, want empty map so GetPosts hits PostgreSQL", got)
	}
	if err := c.DeleteAndTombstone(ctx, 7); err != nil {
		t.Fatalf("DeleteAndTombstone() error = %v", err)
	}
}
