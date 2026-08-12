package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
	"github.com/redis/go-redis/v9"
)

const tombstonesKey = "tombstones"

type Redis struct {
	client *redis.Client
	ttl    time.Duration
}

type value struct {
	ID              int64  `json:"id"`
	AuthorID        int64  `json:"authorId"`
	Content         string `json:"content"`
	MediaURL        string `json:"mediaUrl,omitempty"`
	CreatedAtUnixMs int64  `json:"createdAtUnixMs"`
}

func New(client *redis.Client, ttl time.Duration) *Redis {
	return &Redis{client: client, ttl: ttl}
}

func key(id int64) string {
	return fmt.Sprintf("post:%d", id)
}

func (c *Redis) Set(ctx context.Context, p post.Post) error {
	encoded, err := encode(p)
	if err != nil {
		return fmt.Errorf("marshal cached post: %w", err)
	}
	if err := c.client.Set(ctx, key(p.ID), encoded, c.ttl).Err(); err != nil {
		return fmt.Errorf("cache post: %w", err)
	}
	return nil
}

func (c *Redis) SetMany(ctx context.Context, posts []post.Post) error {
	if len(posts) == 0 {
		return nil
	}
	pipe := c.client.Pipeline()
	for _, p := range posts {
		encoded, err := encode(p)
		if err != nil {
			return fmt.Errorf("marshal cached post %d: %w", p.ID, err)
		}
		pipe.Set(ctx, key(p.ID), encoded, c.ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache posts: %w", err)
	}
	return nil
}

func (c *Redis) GetMany(ctx context.Context, ids []int64) (map[int64]post.Post, error) {
	if len(ids) == 0 {
		return map[int64]post.Post{}, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = key(id)
	}
	values, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("mget posts: %w", err)
	}

	posts := make(map[int64]post.Post, len(ids))
	for i, result := range values {
		if result == nil {
			continue
		}
		raw, ok := result.(string)
		if !ok {
			return nil, fmt.Errorf("cached post %d has unexpected type %T", ids[i], result)
		}
		var cached value
		if err := json.Unmarshal([]byte(raw), &cached); err != nil {
			return nil, fmt.Errorf("decode cached post %d: %w", ids[i], err)
		}
		posts[ids[i]] = post.Post{
			ID:        cached.ID,
			AuthorID:  cached.AuthorID,
			Content:   cached.Content,
			MediaURL:  cached.MediaURL,
			CreatedAt: time.UnixMilli(cached.CreatedAtUnixMs),
		}
	}
	return posts, nil
}

func (c *Redis) DeleteAndTombstone(ctx context.Context, postID int64) error {
	pipe := c.client.TxPipeline()
	pipe.Del(ctx, key(postID))
	pipe.SAdd(ctx, tombstonesKey, strconv.FormatInt(postID, 10))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("invalidate deleted post: %w", err)
	}
	return nil
}

func encode(p post.Post) ([]byte, error) {
	return json.Marshal(value{
		ID:              p.ID,
		AuthorID:        p.AuthorID,
		Content:         p.Content,
		MediaURL:        p.MediaURL,
		CreatedAtUnixMs: p.CreatedAt.UnixMilli(),
	})
}
