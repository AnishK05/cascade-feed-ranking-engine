package candidate

import (
	"context"
	"testing"
)

func TestPostgresLoadSplitsNormalCelebrityAndFollowed(t *testing.T) {
	calls := 0
	store := &Postgres{ids: func(_ context.Context, _ string, _ ...any) ([]int64, error) {
		calls++
		switch calls {
		case 1:
			return []int64{10, 11}, nil
		case 2:
			return []int64{20, 21}, nil
		case 3:
			return []int64{99}, nil
		default:
			t.Fatalf("unexpected query %d", calls)
			return nil, nil
		}
	}}

	got, err := store.Load(context.Background(), 7, 10)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if calls != 3 {
		t.Fatalf("queries = %d, want 3", calls)
	}
	assertIDs(t, got.NormalIDs, []int64{10, 11})
	assertIDs(t, got.CelebrityIDs, []int64{20, 21})
	if _, ok := got.FollowedCelebrities[99]; !ok {
		t.Fatal("followed celebrity 99 is missing")
	}
}

func TestPostgresLoadRejectsNonPositiveLimit(t *testing.T) {
	store := &Postgres{ids: func(context.Context, string, ...any) ([]int64, error) {
		t.Fatal("Load() issued a query for an invalid limit")
		return nil, nil
	}}
	if _, err := store.Load(context.Background(), 1, 0); err == nil {
		t.Fatal("Load() error = nil, want invalid limit error")
	}
}
