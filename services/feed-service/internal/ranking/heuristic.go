// Package ranking implements the in-process heuristic feed ranker.
package ranking

import (
	"math"
	"sort"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
)

type Weights struct {
	Recency    float64
	Engagement float64
	Affinity   float64
	HalfLife   time.Duration
}

type Heuristic struct {
	weights Weights
	now     func() time.Time
}

func NewHeuristic(weights Weights) *Heuristic {
	return &Heuristic{weights: weights, now: time.Now}
}

func (h *Heuristic) Rank(posts []feed.Post, signals map[int64]feed.Signal) []feed.RankedPost {
	now := h.now()
	result := make([]feed.RankedPost, 0, len(posts))
	for _, post := range posts {
		age := now.Sub(post.CreatedAt)
		if age < 0 {
			age = 0
		}
		signal := signals[post.ID]
		recency := math.Exp(-float64(age) / float64(h.weights.HalfLife))
		engagement := math.Log1p(float64(signal.Likes + 2*signal.Comments))
		score := h.weights.Recency*recency +
			h.weights.Engagement*engagement +
			h.weights.Affinity*signal.Affinity
		result = append(result, feed.RankedPost{Post: post, Score: score})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		if !result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].ID > result[j].ID
	})
	return result
}
