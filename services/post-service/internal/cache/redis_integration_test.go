package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
	"github.com/redis/go-redis/v9"
)

func TestRedisIntegrationRoundTrip(t *testing.T) {
	addr := os.Getenv("POST_SERVICE_INTEGRATION_REDIS_ADDR")
	if addr == "" {
		t.Skip("set POST_SERVICE_INTEGRATION_REDIS_ADDR to run Redis integration tests")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis ping: %v", err)
	}

	id := time.Now().UnixNano()
	cache := New(client, time.Minute, 24*time.Hour)
	t.Cleanup(func() {
		_ = client.Del(context.Background(), key(id)).Err()
		_ = client.SRem(context.Background(), tombstonesKey, id).Err()
	})
	want := post.Post{ID: id, AuthorID: 12, Content: "integration", CreatedAt: time.Now().UTC().Truncate(time.Nanosecond)}
	if err := cache.Set(ctx, want); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := cache.GetMany(ctx, []int64{id})
	if err != nil || got[id].Content != want.Content {
		t.Fatalf("GetMany() = (%v, %v)", got, err)
	}
	if err := cache.DeleteAndTombstone(ctx, id); err != nil {
		t.Fatalf("DeleteAndTombstone() error = %v", err)
	}
	member, err := client.SIsMember(ctx, tombstonesKey, id).Result()
	if err != nil || !member {
		t.Fatalf("tombstone membership = %v, %v", member, err)
	}
}
