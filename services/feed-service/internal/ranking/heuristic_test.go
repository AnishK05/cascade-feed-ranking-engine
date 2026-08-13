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
	sum := got[0].Recency + got[0].Engagement + got[0].Affinity
	if math.Abs(got[0].Score-sum) > 1e-12 {
		t.Fatalf("score components %f + %f + %f = %f, want %f",
			got[0].Recency, got[0].Engagement, got[0].Affinity, sum, got[0].Score)
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

func TestRankWeightCombinations(t *testing.T) {
	now := time.Date(2026, 8, 13, 18, 0, 0, 0, time.UTC)
	older := feed.Post{ID: 1, AuthorID: 1, CreatedAt: now.Add(-time.Hour)}
	newer := feed.Post{ID: 2, AuthorID: 2, CreatedAt: now}
	tests := []struct {
		name      string
		weights   Weights
		signals   map[int64]feed.Signal
		wantFirst int64
	}{
		{
			name:      "recency only prefers the newer post",
			weights:   Weights{Recency: 1, HalfLife: time.Hour},
			wantFirst: 2,
		},
		{
			name:      "engagement can beat recency",
			weights:   Weights{Recency: 1, Engagement: 5, HalfLife: time.Hour},
			signals:   map[int64]feed.Signal{1: {Likes: 50}},
			wantFirst: 1,
		},
		{
			name:      "affinity can beat recency",
			weights:   Weights{Recency: 1, Affinity: 5, HalfLife: time.Hour},
			signals:   map[int64]feed.Signal{1: {Affinity: 4}},
			wantFirst: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranker := NewHeuristic(tt.weights)
			ranker.now = func() time.Time { return now }
			got := ranker.Rank([]feed.Post{older, newer}, tt.signals)
			if got[0].ID != tt.wantFirst {
				t.Fatalf("first = %d, want %d (scores [%.4f, %.4f])", got[0].ID, tt.wantFirst, got[0].Score, got[1].Score)
			}
		})
	}
}

func TestRankEmptyInput(t *testing.T) {
	if got := NewHeuristic(Weights{HalfLife: time.Hour}).Rank(nil, nil); len(got) != 0 {
		t.Fatalf("Rank(nil) = %v, want empty", got)
	}
}

func TestRankStaysUnderLatencyBudget(t *testing.T) {
	now := time.Date(2026, 8, 13, 18, 0, 0, 0, time.UTC)
	ranker := NewHeuristic(Weights{Recency: 1, Engagement: 1, Affinity: 1, HalfLife: 12 * time.Hour})
	ranker.now = func() time.Time { return now }
	posts := make([]feed.Post, 500)
	signals := make(map[int64]feed.Signal, 500)
	for i := range posts {
		id := int64(i + 1)
		posts[i] = feed.Post{ID: id, AuthorID: id, CreatedAt: now.Add(-time.Duration(i) * time.Minute)}
		signals[id] = feed.Signal{Likes: int64(i % 11), Comments: int64(i % 3), Affinity: float64(i % 5)}
	}
	started := time.Now()
	got := ranker.Rank(posts, signals)
	elapsed := time.Since(started)
	if len(got) != 500 {
		t.Fatalf("Rank() returned %d posts", len(got))
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("Rank(500) took %s, want < 50ms (Phase 16 ranking budget)", elapsed)
	}
}
