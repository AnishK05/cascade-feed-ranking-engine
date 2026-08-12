package timeline

import (
	"context"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*Redis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedis(client, time.Minute), client
}

func TestFanoutPostIsIdempotentAndTrims(t *testing.T) {
	ctx := context.Background()
	timelines, client := newTestRedis(t)
	for _, post := range []repository.Post{
		{ID: 1, CreatedAtUnixMs: 10},
		{ID: 2, CreatedAtUnixMs: 20},
		{ID: 3, CreatedAtUnixMs: 30},
	} {
		if err := timelines.FanoutPost(ctx, []int64{7, 8}, post, 2, 1); err != nil {
			t.Fatal(err)
		}
	}
	// Kafka redelivery of the same post does not duplicate a ZSET member.
	if err := timelines.FanoutPost(ctx, []int64{7, 8}, repository.Post{ID: 3, CreatedAtUnixMs: 30}, 2, 1); err != nil {
		t.Fatal(err)
	}
	for _, follower := range []string{"7", "8"} {
		values, err := client.ZRange(ctx, "timeline:"+follower, 0, -1).Result()
		if err != nil {
			t.Fatal(err)
		}
		if len(values) != 2 || values[0] != "2" || values[1] != "3" {
			t.Errorf("timeline:%s = %#v, want [2 3]", follower, values)
		}
	}
}

func TestCelebrityPostBackfillTombstoneAndFollowSets(t *testing.T) {
	ctx := context.Background()
	timelines, client := newTestRedis(t)
	for id := int64(1); id <= 3; id++ {
		if err := timelines.AddCelebrityPost(ctx, repository.Post{ID: id, CreatedAtUnixMs: id}, 2); err != nil {
			t.Fatal(err)
		}
	}
	if got := client.ZCard(ctx, celebrityPostsKey).Val(); got != 2 {
		t.Errorf("celebrity post count = %d, want 2", got)
	}
	posts := []repository.Post{{ID: 4, CreatedAtUnixMs: 4}, {ID: 5, CreatedAtUnixMs: 5}}
	if err := timelines.Backfill(ctx, 10, posts, 1); err != nil {
		t.Fatal(err)
	}
	if got := client.ZRange(ctx, "timeline:10", 0, -1).Val(); len(got) != 1 || got[0] != "5" {
		t.Errorf("backfilled timeline = %#v, want [5]", got)
	}
	if err := timelines.AddTombstone(ctx, 5); err != nil {
		t.Fatal(err)
	}
	if err := timelines.AddTombstone(ctx, 5); err != nil {
		t.Fatal(err)
	}
	if got := client.SCard(ctx, tombstonesKey).Val(); got != 1 {
		t.Errorf("tombstone count = %d, want 1", got)
	}
	if err := timelines.AddCelebrityFollow(ctx, 10, 20); err != nil {
		t.Fatal(err)
	}
	if err := timelines.RemoveCelebrityFollow(ctx, 10, 20); err != nil {
		t.Fatal(err)
	}
	if client.SIsMember(ctx, "following:celebrities:10", "20").Val() {
		t.Error("celebrity follow marker was not removed")
	}
}

func TestFollowerCountCache(t *testing.T) {
	ctx := context.Background()
	timelines, _ := newTestRedis(t)
	if _, ok, err := timelines.CachedFollowerCount(ctx, 2); err != nil || ok {
		t.Fatalf("initial cache lookup = ok %v, err %v", ok, err)
	}
	if err := timelines.CacheFollowerCount(ctx, 2, 42); err != nil {
		t.Fatal(err)
	}
	if count, ok, err := timelines.CachedFollowerCount(ctx, 2); err != nil || !ok || count != 42 {
		t.Fatalf("cached count = %d, ok %v, err %v", count, ok, err)
	}
	if err := timelines.InvalidateFollowerCount(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := timelines.CachedFollowerCount(ctx, 2); ok {
		t.Fatal("cache entry still exists after invalidation")
	}
}
