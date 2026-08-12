package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Post struct {
	ID              int64
	CreatedAtUnixMs int64
}

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) FollowerCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	if err := p.pool.QueryRow(ctx,
		`SELECT follower_count FROM public.users WHERE id = $1`, userID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("query follower count for user %d: %w", userID, err)
	}
	return count, nil
}

func (p *Postgres) FollowerIDs(ctx context.Context, followeeID int64) ([]int64, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT follower_id FROM public.follows WHERE followee_id = $1 ORDER BY follower_id`,
		followeeID,
	)
	if err != nil {
		return nil, fmt.Errorf("query followers for user %d: %w", followeeID, err)
	}
	defer rows.Close()

	var followers []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan follower for user %d: %w", followeeID, err)
		}
		followers = append(followers, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate followers for user %d: %w", followeeID, err)
	}
	return followers, nil
}

func (p *Postgres) RecentPosts(ctx context.Context, authorID, limit int64) ([]Post, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, created_at
		FROM public.posts
		WHERE author_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2`, authorID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent posts for user %d: %w", authorID, err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var createdAt time.Time
		if err := rows.Scan(&post.ID, &createdAt); err != nil {
			return nil, fmt.Errorf("scan recent post for user %d: %w", authorID, err)
		}
		post.CreatedAtUnixMs = createdAt.UnixMilli()
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent posts for user %d: %w", authorID, err)
	}
	return posts, nil
}
