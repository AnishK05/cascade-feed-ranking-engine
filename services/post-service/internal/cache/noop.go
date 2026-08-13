package cache

import (
	"context"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
)

// Noop implements Cache without touching Redis. GetPosts therefore always falls
// through to PostgreSQL — the Phase 12 baseline (POST_BYPASS_CACHE=true).
type Noop struct{}

func NewNoop() *Noop {
	return &Noop{}
}

func (Noop) Set(context.Context, post.Post) error { return nil }

func (Noop) SetMany(context.Context, []post.Post) error { return nil }

func (Noop) GetMany(_ context.Context, _ []int64) (map[int64]post.Post, error) {
	return map[int64]post.Post{}, nil
}

func (Noop) DeleteAndTombstone(context.Context, int64) error { return nil }
