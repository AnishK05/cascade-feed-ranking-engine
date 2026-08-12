package ranking

import (
	"math"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
)

func TestEngagementAndAffinityReorderChronologicalCandidates(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	ranker := NewHeuristic(Weights{Recency: 1, Engagement: 2, Affinity: 3, HalfLife: 12 * time.Hour})
	ranker.now = func() time.Time { return now }
	posts := []feed.Post{
		{ID: 2, AuthorID: 2, CreatedAt: now.Add(-time.Minute)},
		{ID: 1, AuthorID: 1, CreatedAt: now.Add(-time.Hour)},
	}
	got := ranker.Rank(posts, map[int64]feed.Signal{1: {Likes: 10, Comments: 2, Affinity: 4}})
	if got[0].ID != 1 {
		t.Fatalf("ranked IDs = [%d, %d], engagement and affinity should reorder chronology", got[0].ID, got[1].ID)
	}
}

func TestRecencyExponentialDecay(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	ranker := NewHeuristic(Weights{Recency: 1, HalfLife: time.Hour})
	ranker.now = func() time.Time { return now }
	got := ranker.Rank([]feed.Post{
		{ID: 1, CreatedAt: now},
		{ID: 2, CreatedAt: now.Add(-time.Hour)},
	}, nil)
	if math.Abs(got[0].Score-1) > 1e-12 || math.Abs(got[1].Score-math.Exp(-1)) > 1e-12 {
		t.Fatalf("scores = [%f, %f], want [1, exp(-1)]", got[0].Score, got[1].Score)
	}
}

func TestDeterministicTieBreaks(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	ranker := NewHeuristic(Weights{HalfLife: time.Hour})
	ranker.now = func() time.Time { return now }
	got := ranker.Rank([]feed.Post{
		{ID: 1, CreatedAt: now.Add(-time.Minute)},
		{ID: 2, CreatedAt: now},
		{ID: 3, CreatedAt: now},
	}, nil)
	want := []int64{3, 2, 1}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("ranked IDs = [%d, %d, %d], want %v", got[0].ID, got[1].ID, got[2].ID, want)
		}
	}
}
