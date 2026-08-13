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

type User struct {
	ID            int64
	FollowerCount int64
	Celebrity     bool
}

type FollowEdge struct {
	FollowerID int64
	FolloweeID int64
}

type PostContent struct {
	ID              int64
	AuthorID        int64
	Content         string
	MediaURL        string
	CreatedAtUnixMs int64
}

func (p *Postgres) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, follower_count, is_celebrity
		FROM public.users
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.FollowerCount, &user.Celebrity); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (p *Postgres) ListFollows(ctx context.Context) ([]FollowEdge, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT follower_id, followee_id
		FROM public.follows
		ORDER BY follower_id, followee_id`)
	if err != nil {
		return nil, fmt.Errorf("list follows: %w", err)
	}
	defer rows.Close()

	var follows []FollowEdge
	for rows.Next() {
		var edge FollowEdge
		if err := rows.Scan(&edge.FollowerID, &edge.FolloweeID); err != nil {
			return nil, fmt.Errorf("scan follow: %w", err)
		}
		follows = append(follows, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate follows: %w", err)
	}
	return follows, nil
}

func (p *Postgres) RecentPostsPerAuthor(ctx context.Context, perAuthor int64) ([]PostContent, error) {
	if perAuthor <= 0 {
		return nil, nil
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, author_id, content, COALESCE(media_url, ''), created_at
		FROM (
			SELECT id, author_id, content, media_url, created_at,
			       ROW_NUMBER() OVER (PARTITION BY author_id ORDER BY created_at DESC, id DESC) AS rn
			FROM public.posts
			WHERE deleted_at IS NULL
		) ranked
		WHERE rn <= $1
		ORDER BY author_id, created_at DESC, id DESC`, perAuthor)
	if err != nil {
		return nil, fmt.Errorf("list recent posts: %w", err)
	}
	defer rows.Close()

	var posts []PostContent
	for rows.Next() {
		var post PostContent
		var createdAt time.Time
		if err := rows.Scan(&post.ID, &post.AuthorID, &post.Content, &post.MediaURL, &createdAt); err != nil {
			return nil, fmt.Errorf("scan recent post: %w", err)
		}
		post.CreatedAtUnixMs = createdAt.UnixMilli()
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent posts: %w", err)
	}
	return posts, nil
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
