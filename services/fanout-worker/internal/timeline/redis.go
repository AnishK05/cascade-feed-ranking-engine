package timeline

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
	"github.com/redis/go-redis/v9"
)

const (
	celebrityPostsKey = "celebrity_posts:global"
	tombstonesKey     = "tombstones"
)

type Redis struct {
	client       *redis.Client
	cacheTTL     time.Duration
	tombstoneTTL time.Duration
}

func NewRedis(client *redis.Client, cacheTTL, tombstoneTTL time.Duration) *Redis {
	if tombstoneTTL <= 0 {
		tombstoneTTL = 24 * time.Hour
	}
	return &Redis{client: client, cacheTTL: cacheTTL, tombstoneTTL: tombstoneTTL}
}

func (r *Redis) CachedFollowerCount(ctx context.Context, userID int64) (int64, bool, error) {
	value, err := r.client.Get(ctx, followerCountKey(userID)).Int64()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get follower-count cache: %w", err)
	}
	return value, true, nil
}

func (r *Redis) CacheFollowerCount(ctx context.Context, userID, count int64) error {
	if err := r.client.Set(ctx, followerCountKey(userID), count, r.cacheTTL).Err(); err != nil {
		return fmt.Errorf("set follower-count cache: %w", err)
	}
	return nil
}

func (r *Redis) InvalidateFollowerCount(ctx context.Context, userID int64) error {
	if err := r.client.Del(ctx, followerCountKey(userID)).Err(); err != nil {
		return fmt.Errorf("invalidate follower-count cache: %w", err)
	}
	return nil
}

func (r *Redis) FanoutPost(ctx context.Context, followerIDs []int64, post repository.Post, maxLen int64, batchSize int) error {
	for start := 0; start < len(followerIDs); start += batchSize {
		end := min(start+batchSize, len(followerIDs))
		pipe := r.client.Pipeline()
		for _, followerID := range followerIDs[start:end] {
			key := timelineKey(followerID)
			// ZADD deliberately makes Kafka redelivery idempotent: a post ID is a unique
			// member, so replay updates its score rather than introducing a duplicate.
			pipe.ZAdd(ctx, key, redis.Z{Score: float64(post.CreatedAtUnixMs), Member: strconv.FormatInt(post.ID, 10)})
			pipe.ZRemRangeByRank(ctx, key, 0, -maxLen-1)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("fan out post batch: %w", err)
		}
	}
	return nil
}

func (r *Redis) AddCelebrityPost(ctx context.Context, post repository.Post, maxLen int64) error {
	pipe := r.client.TxPipeline()
	pipe.ZAdd(ctx, celebrityPostsKey, redis.Z{
		Score:  float64(post.CreatedAtUnixMs),
		Member: strconv.FormatInt(post.ID, 10),
	})
	pipe.ZRemRangeByRank(ctx, celebrityPostsKey, 0, -maxLen-1)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("add celebrity post: %w", err)
	}
	return nil
}

func (r *Redis) AddTombstone(ctx context.Context, postID int64) error {
	pipe := r.client.TxPipeline()
	pipe.SAdd(ctx, tombstonesKey, strconv.FormatInt(postID, 10))
	pipe.Expire(ctx, tombstonesKey, r.tombstoneTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("add post tombstone: %w", err)
	}
	return nil
}

func (r *Redis) Backfill(ctx context.Context, followerID int64, posts []repository.Post, maxLen int64) error {
	if len(posts) == 0 {
		return nil
	}
	key := timelineKey(followerID)
	pipe := r.client.TxPipeline()
	entries := make([]redis.Z, 0, len(posts))
	for _, post := range posts {
		entries = append(entries, redis.Z{
			Score:  float64(post.CreatedAtUnixMs),
			Member: strconv.FormatInt(post.ID, 10),
		})
	}
	pipe.ZAdd(ctx, key, entries...)
	pipe.ZRemRangeByRank(ctx, key, 0, -maxLen-1)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("backfill timeline: %w", err)
	}
	return nil
}

func (r *Redis) AddCelebrityFollow(ctx context.Context, followerID, followeeID int64) error {
	if err := r.client.SAdd(ctx, celebrityFollowingKey(followerID), strconv.FormatInt(followeeID, 10)).Err(); err != nil {
		return fmt.Errorf("add celebrity follow: %w", err)
	}
	return nil
}

func (r *Redis) RemoveCelebrityFollow(ctx context.Context, followerID, followeeID int64) error {
	if err := r.client.SRem(ctx, celebrityFollowingKey(followerID), strconv.FormatInt(followeeID, 10)).Err(); err != nil {
		return fmt.Errorf("remove celebrity follow: %w", err)
	}
	return nil
}

// UserWarmState is one viewer's reconstructed Redis timeline and celebrity-follow set.
type UserWarmState struct {
	UserID        int64
	FollowerCount int64
	TimelinePosts []repository.Post
	CelebrityIDs  []int64
}

// DeleteWarmableKeys removes timeline, celebrity-follow, follower-count, and global
// celebrity-post keys so a cold Redis instance can be rebuilt from Postgres.
func (r *Redis) DeleteWarmableKeys(ctx context.Context) error {
	for _, pattern := range []string{"timeline:*", "following:celebrities:*", "fanout:follower_count:*"} {
		if err := r.deleteByPattern(ctx, pattern); err != nil {
			return err
		}
	}
	if err := r.client.Del(ctx, celebrityPostsKey).Err(); err != nil {
		return fmt.Errorf("delete celebrity posts: %w", err)
	}
	return nil
}

func (r *Redis) deleteByPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return fmt.Errorf("scan %s: %w", pattern, err)
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("delete %s: %w", pattern, err)
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

// WarmUsers writes reconstructed timelines, celebrity-follow sets, and follower counts.
func (r *Redis) WarmUsers(ctx context.Context, states []UserWarmState, maxLen int64, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 100
	}
	for start := 0; start < len(states); start += batchSize {
		end := min(start+batchSize, len(states))
		pipe := r.client.Pipeline()
		for _, state := range states[start:end] {
			timeline := timelineKey(state.UserID)
			pipe.Del(ctx, timeline, celebrityFollowingKey(state.UserID))
			if len(state.TimelinePosts) > 0 {
				entries := make([]redis.Z, 0, len(state.TimelinePosts))
				for _, post := range state.TimelinePosts {
					entries = append(entries, redis.Z{
						Score:  float64(post.CreatedAtUnixMs),
						Member: strconv.FormatInt(post.ID, 10),
					})
				}
				pipe.ZAdd(ctx, timeline, entries...)
				pipe.ZRemRangeByRank(ctx, timeline, 0, -maxLen-1)
			}
			if len(state.CelebrityIDs) > 0 {
				members := make([]any, len(state.CelebrityIDs))
				for i, id := range state.CelebrityIDs {
					members[i] = strconv.FormatInt(id, 10)
				}
				pipe.SAdd(ctx, celebrityFollowingKey(state.UserID), members...)
			}
			pipe.Set(ctx, followerCountKey(state.UserID), state.FollowerCount, r.cacheTTL)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("warm user timelines: %w", err)
		}
	}
	return nil
}

// ReplaceCelebrityPosts overwrites the global celebrity ZSET used at read time.
func (r *Redis) ReplaceCelebrityPosts(ctx context.Context, posts []repository.Post, maxLen int64) error {
	pipe := r.client.TxPipeline()
	pipe.Del(ctx, celebrityPostsKey)
	if len(posts) > 0 {
		entries := make([]redis.Z, 0, len(posts))
		for _, post := range posts {
			entries = append(entries, redis.Z{
				Score:  float64(post.CreatedAtUnixMs),
				Member: strconv.FormatInt(post.ID, 10),
			})
		}
		pipe.ZAdd(ctx, celebrityPostsKey, entries...)
		pipe.ZRemRangeByRank(ctx, celebrityPostsKey, 0, -maxLen-1)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("replace celebrity posts: %w", err)
	}
	return nil
}

func timelineKey(userID int64) string {
	return "timeline:" + strconv.FormatInt(userID, 10)
}

func celebrityFollowingKey(userID int64) string {
	return "following:celebrities:" + strconv.FormatInt(userID, 10)
}

func followerCountKey(userID int64) string {
	return "fanout:follower_count:" + strconv.FormatInt(userID, 10)
}
