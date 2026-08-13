// Package candidate loads bounded feed candidate sets from Redis.
package candidate

import (
	"context"
	"fmt"
	"strconv"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
	"github.com/redis/go-redis/v9"
)

const (
	celebrityPostsKey = "celebrity_posts:global"
	tombstonesKey     = "tombstones"
)

// Redis reads the normal timeline and global celebrity timeline without N+1 calls.
type Redis struct {
	client *redis.Client
}

func NewRedis(client *redis.Client) *Redis {
	return &Redis{client: client}
}

func (r *Redis) Load(ctx context.Context, userID int64, limit int) (feed.CandidateSet, error) {
	if limit <= 0 {
		return feed.CandidateSet{}, fmt.Errorf("candidate limit must be positive")
	}

	pipe := r.client.Pipeline()
	normalCmd := pipe.ZRevRange(ctx, fmt.Sprintf("timeline:%d", userID), 0, int64(limit-1))
	celebrityCmd := pipe.ZRevRange(ctx, celebrityPostsKey, 0, int64(limit-1))
	followedCmd := pipe.SMembers(ctx, fmt.Sprintf("following:celebrities:%d", userID))
	if _, err := pipe.Exec(ctx); err != nil {
		return feed.CandidateSet{}, fmt.Errorf("read candidate sources: %w", err)
	}

	normal, err := parseIDs(normalCmd.Val())
	if err != nil {
		return feed.CandidateSet{}, fmt.Errorf("parse normal candidates: %w", err)
	}
	celebrities, err := parseIDs(celebrityCmd.Val())
	if err != nil {
		return feed.CandidateSet{}, fmt.Errorf("parse celebrity candidates: %w", err)
	}
	followedIDs, err := parseIDs(followedCmd.Val())
	if err != nil {
		return feed.CandidateSet{}, fmt.Errorf("parse followed celebrities: %w", err)
	}

	all := unique(append(append([]int64(nil), normal...), celebrities...))
	if len(all) > 0 {
		members := make([]any, len(all))
		for i, id := range all {
			members[i] = strconv.FormatInt(id, 10)
		}
		tombstoned, err := r.client.SMIsMember(ctx, tombstonesKey, members...).Result()
		if err != nil {
			return feed.CandidateSet{}, fmt.Errorf("read tombstones: %w", err)
		}
		deleted := make(map[int64]struct{})
		for i, value := range tombstoned {
			if value {
				deleted[all[i]] = struct{}{}
			}
		}
		normal = exclude(normal, deleted)
		celebrities = exclude(celebrities, deleted)
	}

	followed := make(map[int64]struct{}, len(followedIDs))
	for _, id := range followedIDs {
		followed[id] = struct{}{}
	}
	return feed.CandidateSet{
		NormalIDs:           normal,
		CelebrityIDs:        celebrities,
		FollowedCelebrities: followed,
	}, nil
}

func parseIDs(values []string) ([]int64, error) {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid post or author ID %q", value)
		}
		ids = append(ids, id)
	}
	return ids, nil
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

func exclude(ids []int64, excluded map[int64]struct{}) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := excluded[id]; !ok {
			result = append(result, id)
		}
	}
	return result
}
