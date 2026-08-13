package cache

import (
	"context"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisSetGetManyAndTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := New(client, 6*time.Hour, 24*time.Hour)
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	posts := []post.Post{
		{ID: 1, AuthorID: 10, Content: "one", CreatedAt: now},
		{ID: 2, AuthorID: 20, Content: "two", MediaURL: "https://example.com/2", CreatedAt: now},
	}

	if err := cache.Set(context.Background(), posts[0]); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := cache.SetMany(context.Background(), posts[1:]); err != nil {
		t.Fatalf("SetMany() error = %v", err)
	}
	got, err := cache.GetMany(context.Background(), []int64{2, 99, 1})
	if err != nil {
		t.Fatalf("GetMany() error = %v", err)
	}
	if len(got) != 2 || got[1].Content != "one" || got[2].MediaURL != posts[1].MediaURL {
		t.Fatalf("GetMany() = %#v", got)
	}
	for _, id := range []int64{1, 2} {
		ttl := server.TTL(key(id))
		if ttl != 6*time.Hour {
			t.Errorf("TTL(post:%d) = %s, want 6h", id, ttl)
		}
	}
}

func TestRedisDeleteAndTombstone(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := New(client, time.Hour, 24*time.Hour)
	p := post.Post{ID: 7, AuthorID: 3, Content: "deleted", CreatedAt: time.Now()}
	if err := cache.Set(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	if err := cache.DeleteAndTombstone(context.Background(), p.ID); err != nil {
		t.Fatalf("DeleteAndTombstone() error = %v", err)
	}
	if server.Exists(key(p.ID)) {
		t.Errorf("%s still exists", key(p.ID))
	}
	member, err := server.SIsMember(tombstonesKey, "7")
	if err != nil {
		t.Fatalf("SIsMember() error = %v", err)
	}
	if !member {
		t.Error("global tombstones set does not contain post ID")
	}
	if ttl := server.TTL(tombstonesKey); ttl != 24*time.Hour {
		t.Errorf("tombstones TTL = %s, want 24h", ttl)
	}
}

func TestRedisGetManyRejectsCorruptJSON(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	server.Set(key(1), "{invalid")

	if _, err := New(client, time.Hour, 24*time.Hour).GetMany(context.Background(), []int64{1}); err == nil {
		t.Fatal("GetMany() error = nil, want corrupt JSON error")
	}
}
