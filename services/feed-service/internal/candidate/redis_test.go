package candidate

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisLoadNormalCelebrityAndTombstones(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	server.ZAdd("timeline:7", 100, "1")
	server.ZAdd("timeline:7", 200, "2")
	server.ZAdd(celebrityPostsKey, 150, "3")
	server.ZAdd(celebrityPostsKey, 250, "4")
	server.SAdd("following:celebrities:7", "99")
	server.SAdd(tombstonesKey, "2", "4")

	got, err := NewRedis(client).Load(ctx, 7, 10)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertIDs(t, got.NormalIDs, []int64{1})
	assertIDs(t, got.CelebrityIDs, []int64{3})
	if _, ok := got.FollowedCelebrities[99]; !ok {
		t.Fatal("followed celebrity 99 is missing")
	}
}

func TestRedisLoadBoundsNewestCandidates(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	server.ZAdd("timeline:1", 1, "1")
	server.ZAdd("timeline:1", 2, "2")
	server.ZAdd("timeline:1", 3, "3")

	got, err := NewRedis(client).Load(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertIDs(t, got.NormalIDs, []int64{3, 2})
}

func TestRedisLoadErrors(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	server.ZAdd("timeline:1", 1, "not-an-id")
	if _, err := NewRedis(client).Load(context.Background(), 1, 10); err == nil {
		t.Fatal("Load() error = nil, want malformed candidate error")
	}
	if _, err := NewRedis(client).Load(context.Background(), 1, 0); err == nil {
		t.Fatal("Load() error = nil, want invalid limit error")
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
