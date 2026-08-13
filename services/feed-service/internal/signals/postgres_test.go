package signals

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRows struct {
	data [][]any
	i    int
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

func (r *fakeRows) Next() bool {
	if r.i >= len(r.data) {
		return false
	}
	r.i++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.data[r.i-1]
	for i, d := range dest {
		switch ptr := d.(type) {
		case *int64:
			*ptr = row[i].(int64)
		case *float64:
			*ptr = row[i].(float64)
		default:
			return fmt.Errorf("unsupported scan destination %T", d)
		}
	}
	return nil
}

type stubDB struct {
	results [][][]any
	calls   int
}

func (s *stubDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	rows := s.results[s.calls]
	s.calls++
	return &fakeRows{data: rows}, nil
}

func TestLoadEmptyPostsSkipsQueries(t *testing.T) {
	db := &stubDB{}
	got, err := NewPostgres(db, time.Hour, 0.5).Load(context.Background(), 1, nil)
	if err != nil || len(got) != 0 || db.calls != 0 {
		t.Fatalf("Load() = (%v, %v) calls=%d", got, err, db.calls)
	}
}

func TestLoadMergesEngagementAndAffinityInTwoQueries(t *testing.T) {
	db := &stubDB{results: [][][]any{
		{{int64(10), int64(3), int64(1)}},
		{{int64(2), float64(7)}},
	}}
	var ops []string
	store := NewPostgres(db, 24*time.Hour, 0.25)
	store.now = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	store.OnQuery = func(op string) { ops = append(ops, op) }

	got, err := store.Load(context.Background(), 9, []feed.Post{
		{ID: 10, AuthorID: 2},
		{ID: 11, AuthorID: 3},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if db.calls != 2 {
		t.Fatalf("queries = %d, want 2", db.calls)
	}
	if len(ops) != 2 || ops[0] != "signals" || ops[1] != "signals" {
		t.Fatalf("OnQuery ops = %v", ops)
	}
	if got[10].Likes != 3 || got[10].Comments != 1 || got[10].Affinity != 7 {
		t.Fatalf("post 10 signals = %+v", got[10])
	}
	if got[11].Likes != 0 || got[11].Affinity != 0.25 {
		t.Fatalf("post 11 should keep default affinity: %+v", got[11])
	}
}
