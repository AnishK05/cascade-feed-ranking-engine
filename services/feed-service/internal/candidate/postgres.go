package candidate

import (
	"context"
	"fmt"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres loads the same three candidate sources as Redis, but from SQL. Used
// when FEED_BYPASS_CACHE=true so the Phase 12 baseline does not touch Redis
// timelines (IMPLEMENTATION_PLAN.md §13.3).
type Postgres struct {
	ids     func(context.Context, string, ...any) ([]int64, error)
	OnQuery func(op string)
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{ids: func(ctx context.Context, sql string, args ...any) ([]int64, error) {
		rows, err := pool.Query(ctx, sql, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := make([]int64, 0)
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			out = append(out, id)
		}
		return out, rows.Err()
	}}
}

func (p *Postgres) Load(ctx context.Context, userID int64, limit int) (feed.CandidateSet, error) {
	if limit <= 0 {
		return feed.CandidateSet{}, fmt.Errorf("candidate limit must be positive")
	}

	normal, err := p.query(ctx, "candidates", `
		SELECT p.id
		FROM public.posts AS p
		JOIN public.follows AS f ON f.followee_id = p.author_id
		JOIN public.users AS u ON u.id = p.author_id
		WHERE f.follower_id = $1
		  AND p.deleted_at IS NULL
		  AND NOT u.is_celebrity
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return feed.CandidateSet{}, fmt.Errorf("load normal candidates: %w", err)
	}

	celebrities, err := p.query(ctx, "candidates", `
		SELECT p.id
		FROM public.posts AS p
		JOIN public.users AS u ON u.id = p.author_id
		WHERE u.is_celebrity
		  AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return feed.CandidateSet{}, fmt.Errorf("load celebrity candidates: %w", err)
	}

	followedIDs, err := p.query(ctx, "candidates", `
		SELECT f.followee_id
		FROM public.follows AS f
		JOIN public.users AS u ON u.id = f.followee_id
		WHERE f.follower_id = $1 AND u.is_celebrity`, userID)
	if err != nil {
		return feed.CandidateSet{}, fmt.Errorf("load followed celebrities: %w", err)
	}

	followed := make(map[int64]struct{}, len(followedIDs))
	for _, id := range followedIDs {
		followed[id] = struct{}{}
	}
	return feed.CandidateSet{
		NormalIDs:           normal,
		CelebrityIDs:        celebrities,
		FollowedCelebrities: followed,
	}, nil
}

func (p *Postgres) query(ctx context.Context, op, sql string, args ...any) ([]int64, error) {
	ids, err := p.ids(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if p.OnQuery != nil {
		p.OnQuery(op)
	}
	return ids, nil
}
