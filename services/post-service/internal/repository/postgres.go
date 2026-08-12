package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("post not found")
	ErrForbidden = errors.New("post belongs to another user")
)

// Postgres stores posts in public.posts. Create and delete each commit their own transaction
// before returning, allowing callers to safely perform non-transactional side effects.
type Postgres struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (r *Postgres) Create(ctx context.Context, authorID int64, content, mediaURL string) (post.Post, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return post.Post{}, fmt.Errorf("begin create post transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var p post.Post
	err = tx.QueryRow(ctx, `
		INSERT INTO public.posts (author_id, content, media_url)
		VALUES ($1, $2, NULLIF($3, ''))
		RETURNING id, author_id, content, COALESCE(media_url, ''), created_at`,
		authorID, content, mediaURL,
	).Scan(&p.ID, &p.AuthorID, &p.Content, &p.MediaURL, &p.CreatedAt)
	if err != nil {
		return post.Post{}, fmt.Errorf("insert post: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return post.Post{}, fmt.Errorf("commit create post: %w", err)
	}
	return p, nil
}

func (r *Postgres) GetByIDs(ctx context.Context, ids []int64) (map[int64]post.Post, error) {
	if len(ids) == 0 {
		return map[int64]post.Post{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, author_id, content, COALESCE(media_url, ''), created_at
		FROM public.posts
		WHERE id = ANY($1) AND deleted_at IS NULL`, ids)
	if err != nil {
		return nil, fmt.Errorf("query posts: %w", err)
	}
	defer rows.Close()

	posts := make(map[int64]post.Post, len(ids))
	for rows.Next() {
		var p post.Post
		if err := rows.Scan(&p.ID, &p.AuthorID, &p.Content, &p.MediaURL, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		posts[p.ID] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return posts, nil
}

func (r *Postgres) Delete(ctx context.Context, postID, requestingUserID int64) (int64, time.Time, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("begin delete post transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var authorID int64
	err = tx.QueryRow(ctx, `
		SELECT author_id FROM public.posts
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`, postID).Scan(&authorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, time.Time{}, ErrNotFound
	}
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("select post for delete: %w", err)
	}
	if authorID != requestingUserID {
		return 0, time.Time{}, ErrForbidden
	}

	var deletedAt time.Time
	if err := tx.QueryRow(ctx, `
		UPDATE public.posts SET deleted_at = now()
		WHERE id = $1
		RETURNING deleted_at`, postID).Scan(&deletedAt); err != nil {
		return 0, time.Time{}, fmt.Errorf("soft delete post: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, time.Time{}, fmt.Errorf("commit delete post: %w", err)
	}
	return authorID, deletedAt, nil
}
