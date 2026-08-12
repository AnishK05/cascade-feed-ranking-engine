// Package hydrator implements cache-aside post hydration.
package hydrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
	"github.com/redis/go-redis/v9"
)

type RedisPost struct {
	redis *redis.Client
	posts postv1.PostServiceClient
	ttl   time.Duration
}

type cachedPost struct {
	ID              int64  `json:"id"`
	AuthorID        int64  `json:"authorId"`
	Content         string `json:"content"`
	MediaURL        string `json:"mediaUrl,omitempty"`
	CreatedAtUnixMs int64  `json:"createdAtUnixMs"`
}

func NewRedisPost(redisClient *redis.Client, postClient postv1.PostServiceClient, ttl time.Duration) *RedisPost {
	return &RedisPost{redis: redisClient, posts: postClient, ttl: ttl}
}

// Hydrate uses one Redis MGET and, when needed, exactly one PostService.GetPosts call.
func (h *RedisPost) Hydrate(ctx context.Context, ids []int64) (map[int64]feed.Post, error) {
	ids = unique(ids)
	if len(ids) == 0 {
		return map[int64]feed.Post{}, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = "post:" + strconv.FormatInt(id, 10)
	}
	values, err := h.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("read post cache: %w", err)
	}

	result := make(map[int64]feed.Post, len(ids))
	missing := make([]int64, 0)
	for i, value := range values {
		if value == nil {
			missing = append(missing, ids[i])
			continue
		}
		raw, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("cached post %d has unexpected type %T", ids[i], value)
		}
		var cached cachedPost
		if err := json.Unmarshal([]byte(raw), &cached); err != nil {
			return nil, fmt.Errorf("decode cached post %d: %w", ids[i], err)
		}
		if cached.ID != ids[i] || cached.AuthorID <= 0 || cached.CreatedAtUnixMs <= 0 {
			return nil, fmt.Errorf("cached post %d is invalid", ids[i])
		}
		result[ids[i]] = fromCache(cached)
	}

	if len(missing) == 0 {
		return result, nil
	}
	response, err := h.posts.GetPosts(ctx, &postv1.GetPostsRequest{PostIds: missing})
	if err != nil {
		return nil, fmt.Errorf("batch hydrate posts: %w", err)
	}
	backfill := make([]cachedPost, 0, len(response.GetPosts()))
	missingSet := make(map[int64]struct{}, len(missing))
	for _, id := range missing {
		missingSet[id] = struct{}{}
	}
	for _, post := range response.GetPosts() {
		if post == nil {
			continue
		}
		if _, ok := missingSet[post.GetId()]; !ok || post.GetId() <= 0 ||
			post.GetAuthorId() <= 0 || post.GetCreatedAtUnixMs() <= 0 {
			return nil, fmt.Errorf("Post Service returned an invalid or unrequested post")
		}
		cached := cachedPost{
			ID:              post.GetId(),
			AuthorID:        post.GetAuthorId(),
			Content:         post.GetContent(),
			MediaURL:        post.GetMediaUrl(),
			CreatedAtUnixMs: post.GetCreatedAtUnixMs(),
		}
		result[cached.ID] = fromCache(cached)
		backfill = append(backfill, cached)
		delete(missingSet, cached.ID)
	}
	if err := h.backfill(ctx, backfill); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *RedisPost) backfill(ctx context.Context, posts []cachedPost) error {
	if len(posts) == 0 {
		return nil
	}
	pipe := h.redis.Pipeline()
	for _, post := range posts {
		value, err := json.Marshal(post)
		if err != nil {
			return fmt.Errorf("encode post cache backfill: %w", err)
		}
		pipe.Set(ctx, "post:"+strconv.FormatInt(post.ID, 10), value, h.ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("backfill post cache: %w", err)
	}
	return nil
}

func fromCache(post cachedPost) feed.Post {
	return feed.Post{
		ID:        post.ID,
		AuthorID:  post.AuthorID,
		Content:   post.Content,
		MediaURL:  post.MediaURL,
		CreatedAt: time.UnixMilli(post.CreatedAtUnixMs).UTC(),
	}
}

func unique(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
