package hydrator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

type fakePostClient struct {
	calls int
	ids   []int64
	resp  *postv1.GetPostsResponse
	err   error
}

func (f *fakePostClient) CreatePost(context.Context, *postv1.CreatePostRequest, ...grpc.CallOption) (*postv1.CreatePostResponse, error) {
	panic("unexpected CreatePost call")
}
func (f *fakePostClient) DeletePost(context.Context, *postv1.DeletePostRequest, ...grpc.CallOption) (*postv1.DeletePostResponse, error) {
	panic("unexpected DeletePost call")
}
func (f *fakePostClient) GetPosts(_ context.Context, req *postv1.GetPostsRequest, _ ...grpc.CallOption) (*postv1.GetPostsResponse, error) {
	f.calls++
	f.ids = append([]int64(nil), req.GetPostIds()...)
	return f.resp, f.err
}

func TestHydrateCacheHitsMissesOneBatchAndBackfill(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	now := time.Now().UTC().Truncate(time.Millisecond)
	cached, _ := json.Marshal(cachedPost{ID: 1, AuthorID: 10, Content: "hit", CreatedAtUnixMs: now.UnixMilli()})
	server.Set("post:1", string(cached))
	posts := &fakePostClient{resp: &postv1.GetPostsResponse{Posts: []*postv1.Post{
		{Id: 2, AuthorId: 20, Content: "miss 2", CreatedAtUnixMs: now.Add(-time.Minute).UnixMilli()},
		{Id: 3, AuthorId: 30, Content: "miss 3", CreatedAtUnixMs: now.Add(-2 * time.Minute).UnixMilli()},
	}}}

	got, err := NewRedisPost(client, posts, time.Hour).Hydrate(context.Background(), []int64{1, 2, 3, 2})
	if err != nil {
		t.Fatalf("Hydrate() error = %v", err)
	}
	if len(got) != 3 || got[1].Content != "hit" || posts.calls != 1 {
		t.Fatalf("Hydrate() = %+v, Post Service calls = %d", got, posts.calls)
	}
	assertIDs(t, posts.ids, []int64{2, 3})
	for _, id := range []string{"post:2", "post:3"} {
		if !server.Exists(id) || server.TTL(id) != time.Hour {
			t.Errorf("%s was not backfilled with one-hour TTL", id)
		}
	}
}

func TestHydrateAllHitsAvoidsPostService(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	value, _ := json.Marshal(cachedPost{ID: 1, AuthorID: 2, CreatedAtUnixMs: 1})
	server.Set("post:1", string(value))
	posts := &fakePostClient{err: errors.New("must not be called")}
	if _, err := NewRedisPost(client, posts, time.Hour).Hydrate(context.Background(), []int64{1}); err != nil {
		t.Fatalf("Hydrate() error = %v", err)
	}
	if posts.calls != 0 {
		t.Fatalf("Post Service calls = %d, want 0", posts.calls)
	}
}

func TestHydrateDependencyAndCorruptCacheErrors(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	server.Set("post:1", "{bad")
	h := NewRedisPost(client, &fakePostClient{}, time.Hour)
	if _, err := h.Hydrate(context.Background(), []int64{1}); err == nil {
		t.Fatal("Hydrate() error = nil, want corrupt cache error")
	}
	server.Del("post:1")
	h = NewRedisPost(client, &fakePostClient{err: errors.New("down")}, time.Hour)
	if _, err := h.Hydrate(context.Background(), []int64{1}); err == nil {
		t.Fatal("Hydrate() error = nil, want Post Service error")
	}
}

func assertIDs(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("IDs = %v, want %v", got, want)
		}
	}
}
