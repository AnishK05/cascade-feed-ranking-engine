// Package warm rebuilds Redis timeline caches from Postgres after a cold start.
//
// This is the second warming mechanism in IMPLEMENTATION_PLAN.md §7.3: new-follow
// backfill happens online in the Fanout Worker, while this batch job pre-populates
// every viewer's `timeline:{userId}`, celebrity-follow sets, and the global celebrity
// ZSET so a fresh Redis (or a Phase 12 benchmark reset) does not force every first
// feed read to miss.
package warm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/fanout"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/timeline"
	"github.com/redis/go-redis/v9"
)

type Store interface {
	ListUsers(context.Context) ([]repository.User, error)
	ListFollows(context.Context) ([]repository.FollowEdge, error)
	RecentPostsPerAuthor(context.Context, int64) ([]repository.PostContent, error)
}

type TimelineCache interface {
	DeleteWarmableKeys(context.Context) error
	WarmUsers(context.Context, []timeline.UserWarmState, int64, int) error
	ReplaceCelebrityPosts(context.Context, []repository.Post, int64) error
}

type Settings struct {
	CelebrityFollowerThreshold int64
	MaxTimelineLen             int64
	BackfillCount              int64
	BatchSize                  int
	PostCacheTTL               time.Duration
	Reset                      bool
	WarmPosts                  bool
}

type Result struct {
	Users          int
	Timelines      int
	CelebrityPosts int
	PostCache      int
}

type cachedPost struct {
	ID              int64  `json:"id"`
	AuthorID        int64  `json:"authorId"`
	Content         string `json:"content"`
	MediaURL        string `json:"mediaUrl,omitempty"`
	CreatedAtUnixMs int64  `json:"createdAtUnixMs"`
}

type Warmer struct {
	store     Store
	timelines TimelineCache
	posts     *redis.Client
	settings  Settings
}

func New(store Store, timelines TimelineCache, posts *redis.Client, settings Settings) *Warmer {
	if settings.BatchSize <= 0 {
		settings.BatchSize = 100
	}
	if settings.MaxTimelineLen <= 0 {
		settings.MaxTimelineLen = 1000
	}
	if settings.BackfillCount <= 0 {
		settings.BackfillCount = 20
	}
	if settings.PostCacheTTL <= 0 {
		settings.PostCacheTTL = 6 * time.Hour
	}
	return &Warmer{store: store, timelines: timelines, posts: posts, settings: settings}
}

func (w *Warmer) Run(ctx context.Context) (Result, error) {
	users, err := w.store.ListUsers(ctx)
	if err != nil {
		return Result{}, err
	}
	follows, err := w.store.ListFollows(ctx)
	if err != nil {
		return Result{}, err
	}
	posts, err := w.store.RecentPostsPerAuthor(ctx, maxInt64(w.settings.BackfillCount, w.settings.MaxTimelineLen))
	if err != nil {
		return Result{}, err
	}

	postsByAuthor := make(map[int64][]repository.PostContent, len(users))
	contentByID := make(map[int64]repository.PostContent, len(posts))
	for _, post := range posts {
		postsByAuthor[post.AuthorID] = append(postsByAuthor[post.AuthorID], post)
		contentByID[post.ID] = post
	}
	celebrities := make(map[int64]struct{})
	for _, user := range users {
		if user.Celebrity || !fanout.ShouldFanoutOnWrite(user.FollowerCount, w.settings.CelebrityFollowerThreshold) {
			celebrities[user.ID] = struct{}{}
		}
	}
	followeesByFollower := make(map[int64][]int64, len(users))
	for _, edge := range follows {
		followeesByFollower[edge.FollowerID] = append(followeesByFollower[edge.FollowerID], edge.FolloweeID)
	}

	states := make([]timeline.UserWarmState, 0, len(users))
	warmedPosts := make(map[int64]repository.PostContent)
	remember := func(posts []repository.Post) {
		for _, post := range posts {
			if content, ok := contentByID[post.ID]; ok {
				warmedPosts[post.ID] = content
			}
		}
	}
	for _, user := range users {
		state := timeline.UserWarmState{UserID: user.ID, FollowerCount: user.FollowerCount}
		var normal []repository.PostContent
		for _, followeeID := range followeesByFollower[user.ID] {
			if _, celebrity := celebrities[followeeID]; celebrity {
				state.CelebrityIDs = append(state.CelebrityIDs, followeeID)
				continue
			}
			normal = append(normal, postsByAuthor[followeeID]...)
		}
		state.TimelinePosts = newestPosts(normal, w.settings.MaxTimelineLen)
		remember(state.TimelinePosts)
		states = append(states, state)
	}

	celebrityPosts := make([]repository.PostContent, 0)
	for authorID := range celebrities {
		celebrityPosts = append(celebrityPosts, postsByAuthor[authorID]...)
	}
	celebrityNewest := newestContents(celebrityPosts, w.settings.MaxTimelineLen)
	remember(toTimelinePosts(celebrityNewest))

	if w.settings.Reset {
		if err := w.timelines.DeleteWarmableKeys(ctx); err != nil {
			return Result{}, err
		}
	}
	if err := w.timelines.WarmUsers(ctx, states, w.settings.MaxTimelineLen, w.settings.BatchSize); err != nil {
		return Result{}, err
	}
	if err := w.timelines.ReplaceCelebrityPosts(ctx, toTimelinePosts(celebrityNewest), w.settings.MaxTimelineLen); err != nil {
		return Result{}, err
	}

	result := Result{
		Users:          len(users),
		Timelines:      len(states),
		CelebrityPosts: len(celebrityNewest),
	}
	if w.settings.WarmPosts && w.posts != nil {
		contents := make([]repository.PostContent, 0, len(warmedPosts))
		for _, post := range warmedPosts {
			contents = append(contents, post)
		}
		if err := w.writePostCache(ctx, contents); err != nil {
			return Result{}, err
		}
		result.PostCache = len(contents)
	}
	return result, nil
}

func (w *Warmer) writePostCache(ctx context.Context, posts []repository.PostContent) error {
	if len(posts) == 0 {
		return nil
	}
	pipe := w.posts.Pipeline()
	for _, post := range posts {
		encoded, err := json.Marshal(cachedPost{
			ID:              post.ID,
			AuthorID:        post.AuthorID,
			Content:         post.Content,
			MediaURL:        post.MediaURL,
			CreatedAtUnixMs: post.CreatedAtUnixMs,
		})
		if err != nil {
			return fmt.Errorf("encode post cache %d: %w", post.ID, err)
		}
		pipe.Set(ctx, "post:"+strconv.FormatInt(post.ID, 10), encoded, w.settings.PostCacheTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("warm post cache: %w", err)
	}
	return nil
}

func newestPosts(posts []repository.PostContent, limit int64) []repository.Post {
	return toTimelinePosts(newestContents(posts, limit))
}

func newestContents(posts []repository.PostContent, limit int64) []repository.PostContent {
	if len(posts) == 0 || limit <= 0 {
		return nil
	}
	sorted := append([]repository.PostContent(nil), posts...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].CreatedAtUnixMs != sorted[j].CreatedAtUnixMs {
			return sorted[i].CreatedAtUnixMs > sorted[j].CreatedAtUnixMs
		}
		return sorted[i].ID > sorted[j].ID
	})
	if int64(len(sorted)) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

func toTimelinePosts(posts []repository.PostContent) []repository.Post {
	result := make([]repository.Post, 0, len(posts))
	for _, post := range posts {
		result = append(result, repository.Post{ID: post.ID, CreatedAtUnixMs: post.CreatedAtUnixMs})
	}
	return result
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
