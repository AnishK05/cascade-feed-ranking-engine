package hydrator

import (
	"context"
	"fmt"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres hydrates post bodies with one SQL query and never touches Redis or
// Post Service. Used when FEED_BYPASS_CACHE=true (IMPLEMENTATION_PLAN.md §13.3).
type Postgres struct {
	load    func(context.Context, []int64) (map[int64]feed.Post, error)
	OnQuery func(op string)
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{load: func(ctx context.Context, ids []int64) (map[int64]feed.Post, error) {
		rows, err := pool.Query(ctx, `
			SELECT id, author_id, content, COALESCE(media_url, ''), created_at
			FROM public.posts
			WHERE id = ANY($1) AND deleted_at IS NULL`, ids)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		posts := make(map[int64]feed.Post, len(ids))
		for rows.Next() {
			var post feed.Post
			if err := rows.Scan(&post.ID, &post.AuthorID, &post.Content, &post.MediaURL, &post.CreatedAt); err != nil {
				return nil, err
			}
			posts[post.ID] = post
		}
		return posts, rows.Err()
	}}
}

func (h *Postgres) Hydrate(ctx context.Context, ids []int64) (map[int64]feed.Post, int, int, error) {
	ids = unique(ids)
	if len(ids) == 0 {
		return map[int64]feed.Post{}, 0, 0, nil
	}
	posts, err := h.load(ctx, ids)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("hydrate posts from postgres: %w", err)
	}
	if h.OnQuery != nil {
		h.OnQuery("hydrate")
	}
	for id, post := range posts {
		post.CreatedAt = post.CreatedAt.UTC()
		posts[id] = post
	}
	return posts, 0, len(ids), nil
}
