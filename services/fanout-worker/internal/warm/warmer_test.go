package warm

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/timeline"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeStore struct {
	users   []repository.User
	follows []repository.FollowEdge
	posts   []repository.PostContent
}

func (f fakeStore) ListUsers(context.Context) ([]repository.User, error) {
	return f.users, nil
}
func (f fakeStore) ListFollows(context.Context) ([]repository.FollowEdge, error) {
	return f.follows, nil
}
func (f fakeStore) RecentPostsPerAuthor(context.Context, int64) ([]repository.PostContent, error) {
	return f.posts, nil
}

func TestWarmerRebuildsTimelinesCelebritySetAndPostCache(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	server.ZAdd("timeline:99", 1, "old")
	server.SAdd("following:celebrities:99", "old")

	store := fakeStore{
		users: []repository.User{
			{ID: 1, FollowerCount: 1},
			{ID: 2, FollowerCount: 1},
			{ID: 3, FollowerCount: 10, Celebrity: true},
		},
		follows: []repository.FollowEdge{
			{FollowerID: 1, FolloweeID: 2},
			{FollowerID: 1, FolloweeID: 3},
		},
		posts: []repository.PostContent{
			{ID: 20, AuthorID: 2, Content: "normal older", CreatedAtUnixMs: 20},
			{ID: 21, AuthorID: 2, Content: "normal newer", CreatedAtUnixMs: 21},
			{ID: 30, AuthorID: 3, Content: "celebrity", CreatedAtUnixMs: 30},
		},
	}
	warmer := New(store, timeline.NewRedis(client, time.Minute, 24*time.Hour), client, Settings{
		CelebrityFollowerThreshold: 10,
		MaxTimelineLen:             10,
		BackfillCount:              20,
		Reset:                      true,
		WarmPosts:                  true,
		PostCacheTTL:               time.Hour,
	})
	result, err := warmer.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Users != 3 || result.Timelines != 3 || result.CelebrityPosts != 1 || result.PostCache != 3 {
		t.Fatalf("result = %+v", result)
	}
	if server.Exists("timeline:99") || server.Exists("following:celebrities:99") {
		t.Fatal("reset did not delete stale warmable keys")
	}
	got := client.ZRevRange(ctx, "timeline:1", 0, -1).Val()
	if len(got) != 2 || got[0] != "21" || got[1] != "20" {
		t.Fatalf("viewer timeline = %#v, want newest normal posts", got)
	}
	if !client.SIsMember(ctx, "following:celebrities:1", "3").Val() {
		t.Fatal("celebrity follow was not warmed")
	}
	if got := client.ZRange(ctx, "celebrity_posts:global", 0, -1).Val(); len(got) != 1 || got[0] != "30" {
		t.Fatalf("celebrity posts = %#v", got)
	}
	raw, err := client.Get(ctx, "post:21").Result()
	if err != nil {
		t.Fatalf("post cache: %v", err)
	}
	var cached cachedPost
	if err := json.Unmarshal([]byte(raw), &cached); err != nil || cached.Content != "normal newer" {
		t.Fatalf("cached post = %#v, err %v", cached, err)
	}
	if ttl := server.TTL("post:21"); ttl != time.Hour {
		t.Errorf("post cache TTL = %s, want 1h", ttl)
	}
}

func TestWarmerTrimsTimelinesAndSkipsCelebrityBackfill(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := fakeStore{
		users: []repository.User{
			{ID: 1, FollowerCount: 0},
			{ID: 2, FollowerCount: 1},
		},
		follows: []repository.FollowEdge{{FollowerID: 1, FolloweeID: 2}},
		posts: []repository.PostContent{
			{ID: 1, AuthorID: 2, CreatedAtUnixMs: 1},
			{ID: 2, AuthorID: 2, CreatedAtUnixMs: 2},
			{ID: 3, AuthorID: 2, CreatedAtUnixMs: 3},
		},
	}
	_, err := New(store, timeline.NewRedis(client, time.Minute, 24*time.Hour), client, Settings{
		CelebrityFollowerThreshold: 10,
		MaxTimelineLen:             2,
		Reset:                      true,
	}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := client.ZRange(context.Background(), "timeline:1", 0, -1).Val()
	if len(got) != 2 || got[0] != "2" || got[1] != "3" {
		t.Fatalf("trimmed timeline = %#v, want [2 3]", got)
	}
	if client.Exists(context.Background(), "following:celebrities:1").Val() != 0 {
		t.Fatal("normal follow should not create a celebrity-follow set")
	}
}

func TestWarmerUsesFollowerCountThresholdForCelebrity(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := fakeStore{
		users: []repository.User{
			{ID: 1, FollowerCount: 0},
			{ID: 2, FollowerCount: 10},
		},
		follows: []repository.FollowEdge{{FollowerID: 1, FolloweeID: 2}},
		posts:   []repository.PostContent{{ID: 9, AuthorID: 2, Content: "celeb", CreatedAtUnixMs: 9}},
	}
	_, err := New(store, timeline.NewRedis(client, time.Minute, 24*time.Hour), client, Settings{
		CelebrityFollowerThreshold: 10,
		MaxTimelineLen:             10,
		Reset:                      true,
		WarmPosts:                  true,
		PostCacheTTL:               time.Minute,
	}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if client.Exists(context.Background(), "timeline:1").Val() != 0 {
		t.Fatal("celebrity followee posts must not be written into the normal timeline")
	}
	if !client.SIsMember(context.Background(), "following:celebrities:1", strconv.FormatInt(2, 10)).Val() {
		t.Fatal("threshold celebrity was not recorded")
	}
}
