// Package signals loads ranking features from Postgres in bounded, batched queries.
package signals

import (
	"context"
	"fmt"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
	"github.com/jackc/pgx/v5"
)

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

type Postgres struct {
	db              querier
	affinityWindow  time.Duration
	affinityDefault float64
	now             func() time.Time
}

func NewPostgres(db querier, affinityWindow time.Duration, affinityDefault float64) *Postgres {
	return &Postgres{
		db:              db,
		affinityWindow:  affinityWindow,
		affinityDefault: affinityDefault,
		now:             time.Now,
	}
}

// Load uses one grouped query for post engagement and one grouped query for viewer-author
// affinity. It never issues per-post or per-author queries.
func (p *Postgres) Load(ctx context.Context, viewerID int64, posts []feed.Post) (map[int64]feed.Signal, error) {
	result := make(map[int64]feed.Signal, len(posts))
	if len(posts) == 0 {
		return result, nil
	}
	postIDs := make([]int64, 0, len(posts))
	authorSet := make(map[int64]struct{})
	for _, post := range posts {
		postIDs = append(postIDs, post.ID)
		authorSet[post.AuthorID] = struct{}{}
		result[post.ID] = feed.Signal{Affinity: p.affinityDefault}
	}

	rows, err := p.db.Query(ctx, `
		SELECT post_id,
		       COUNT(*) FILTER (WHERE type = 'like')::bigint,
		       COUNT(*) FILTER (WHERE type = 'comment')::bigint
		FROM public.engagements
		WHERE post_id = ANY($1)
		GROUP BY post_id`, postIDs)
	if err != nil {
		return nil, fmt.Errorf("load engagement counts: %w", err)
	}
	for rows.Next() {
		var postID, likes, comments int64
		if err := rows.Scan(&postID, &likes, &comments); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan engagement counts: %w", err)
		}
		signal := result[postID]
		signal.Likes = likes
		signal.Comments = comments
		result[postID] = signal
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate engagement counts: %w", err)
	}
	rows.Close()

	authorIDs := make([]int64, 0, len(authorSet))
	for authorID := range authorSet {
		authorIDs = append(authorIDs, authorID)
	}
	rows, err = p.db.Query(ctx, `
		SELECT posts.author_id, COUNT(*)::double precision
		FROM public.engagements AS engagements
		JOIN public.posts AS posts ON posts.id = engagements.post_id
		WHERE engagements.user_id = $1
		  AND engagements.created_at >= $2
		  AND posts.author_id = ANY($3)
		GROUP BY posts.author_id`, viewerID, p.now().Add(-p.affinityWindow), authorIDs)
	if err != nil {
		return nil, fmt.Errorf("load viewer-author affinity: %w", err)
	}
	affinity := make(map[int64]float64, len(authorIDs))
	for rows.Next() {
		var authorID int64
		var value float64
		if err := rows.Scan(&authorID, &value); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan viewer-author affinity: %w", err)
		}
		affinity[authorID] = value
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate viewer-author affinity: %w", err)
	}
	rows.Close()

	for _, post := range posts {
		if value, ok := affinity[post.AuthorID]; ok {
			signal := result[post.ID]
			signal.Affinity = value
			result[post.ID] = signal
		}
	}
	return result, nil
}
