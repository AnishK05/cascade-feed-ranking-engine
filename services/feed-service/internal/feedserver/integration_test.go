package feedserver

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	feedv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/feed/v1"
	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/candidate"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/hydrator"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/ranking"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/signals"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type integrationPostServer struct {
	postv1.UnimplementedPostServiceServer
	mu       sync.Mutex
	posts    map[int64]*postv1.Post
	requests [][]int64
}

func (s *integrationPostServer) GetPosts(_ context.Context, req *postv1.GetPostsRequest) (*postv1.GetPostsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, append([]int64(nil), req.GetPostIds()...))
	response := &postv1.GetPostsResponse{}
	for _, id := range req.GetPostIds() {
		if post := s.posts[id]; post != nil {
			response.Posts = append(response.Posts, post)
		}
	}
	return response, nil
}

func TestFeedIntegration(t *testing.T) {
	databaseURL := os.Getenv("FEED_SERVICE_INTEGRATION_DATABASE_URL")
	redisAddr := os.Getenv("FEED_SERVICE_INTEGRATION_REDIS_ADDR")
	if databaseURL == "" || redisAddr == "" {
		t.Skip("set FEED_SERVICE_INTEGRATION_DATABASE_URL and FEED_SERVICE_INTEGRATION_REDIS_ADDR")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = redisClient.Close() })
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	var viewerID, normalAuthorID, celebrityAuthorID, otherCelebrityID int64
	for i, target := range []*int64{&viewerID, &normalAuthorID, &celebrityAuthorID, &otherCelebrityID} {
		err := pool.QueryRow(ctx,
			`INSERT INTO public.users (username, display_name) VALUES ($1, $2) RETURNING id`,
			"feed-integration-"+suffix+"-"+strconv.Itoa(i), "Feed Integration",
		).Scan(target)
		if err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	var normalPostID, celebrityPostID, otherPostID int64
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, value := range []struct {
		target   *int64
		authorID int64
		content  string
		created  time.Time
	}{
		{&normalPostID, normalAuthorID, "engaging older post", now.Add(-2 * time.Hour)},
		{&celebrityPostID, celebrityAuthorID, "followed celebrity", now.Add(-time.Minute)},
		{&otherPostID, otherCelebrityID, "unfollowed celebrity", now},
	} {
		err := pool.QueryRow(ctx,
			`INSERT INTO public.posts (author_id, content, created_at) VALUES ($1, $2, $3) RETURNING id`,
			value.authorID, value.content, value.created,
		).Scan(value.target)
		if err != nil {
			t.Fatalf("insert post: %v", err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.engagements WHERE post_id = ANY($1)`, []int64{normalPostID, celebrityPostID, otherPostID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.posts WHERE id = ANY($1)`, []int64{normalPostID, celebrityPostID, otherPostID})
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.users WHERE id = ANY($1)`, []int64{viewerID, normalAuthorID, celebrityAuthorID, otherCelebrityID})
	})
	for i := 0; i < 8; i++ {
		if _, err := pool.Exec(ctx,
			`INSERT INTO public.engagements (post_id, user_id, type, created_at) VALUES ($1, $2, 'like', $3)`,
			normalPostID, viewerID, now,
		); err != nil {
			t.Fatalf("insert engagement: %v", err)
		}
	}

	timelineKey := "timeline:" + strconv.FormatInt(viewerID, 10)
	followedKey := "following:celebrities:" + strconv.FormatInt(viewerID, 10)
	cacheKeys := []string{"post:" + strconv.FormatInt(normalPostID, 10), "post:" + strconv.FormatInt(otherPostID, 10)}
	celebrityCacheKey := "post:" + strconv.FormatInt(celebrityPostID, 10)
	previousCelebrityPosts, err := redisClient.ZRangeWithScores(ctx, "celebrity_posts:global", 0, -1).Result()
	if err != nil {
		t.Fatalf("snapshot celebrity candidates: %v", err)
	}
	if err := redisClient.Del(ctx, timelineKey, followedKey, "celebrity_posts:global", celebrityCacheKey).Err(); err != nil {
		t.Fatalf("clear integration Redis keys: %v", err)
	}
	if err := redisClient.Del(ctx, cacheKeys...).Err(); err != nil {
		t.Fatalf("clear integration post cache: %v", err)
	}
	if err := redisClient.SRem(ctx, "tombstones", normalPostID, celebrityPostID, otherPostID).Err(); err != nil {
		t.Fatalf("clear integration tombstones: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = redisClient.Del(cleanupCtx, timelineKey, followedKey).Err()
		_ = redisClient.Del(cleanupCtx, "celebrity_posts:global").Err()
		if len(previousCelebrityPosts) > 0 {
			_ = redisClient.ZAdd(cleanupCtx, "celebrity_posts:global", previousCelebrityPosts...).Err()
		}
		_ = redisClient.Del(cleanupCtx, append(cacheKeys, celebrityCacheKey)...).Err()
	})
	if err := redisClient.ZAdd(ctx, timelineKey, redis.Z{Score: float64(normalPostID), Member: normalPostID}).Err(); err != nil {
		t.Fatalf("seed normal timeline: %v", err)
	}
	if err := redisClient.ZAdd(ctx, "celebrity_posts:global",
		redis.Z{Score: float64(now.Add(-time.Minute).UnixMilli()), Member: celebrityPostID},
		redis.Z{Score: float64(now.UnixMilli()), Member: otherPostID},
	).Err(); err != nil {
		t.Fatalf("seed celebrity timeline: %v", err)
	}
	if err := redisClient.SAdd(ctx, followedKey, celebrityAuthorID).Err(); err != nil {
		t.Fatalf("seed followed celebrities: %v", err)
	}
	for _, post := range []*postv1.Post{
		{Id: normalPostID, AuthorId: normalAuthorID, Content: "engaging older post", CreatedAtUnixMs: now.Add(-2 * time.Hour).UnixMilli()},
		{Id: otherPostID, AuthorId: otherCelebrityID, Content: "unfollowed celebrity", CreatedAtUnixMs: now.UnixMilli()},
	} {
		value, _ := json.Marshal(map[string]any{
			"id": post.Id, "authorId": post.AuthorId, "content": post.Content,
			"createdAtUnixMs": post.CreatedAtUnixMs,
		})
		if err := redisClient.Set(ctx, "post:"+strconv.FormatInt(post.Id, 10), value, time.Hour).Err(); err != nil {
			t.Fatalf("seed cache: %v", err)
		}
	}

	fakePost := &integrationPostServer{posts: map[int64]*postv1.Post{
		celebrityPostID: {
			Id: celebrityPostID, AuthorId: celebrityAuthorID, Content: "followed celebrity",
			CreatedAtUnixMs: now.Add(-time.Minute).UnixMilli(),
		},
	}}
	postListener := bufconn.Listen(1024 * 1024)
	postGRPC := grpc.NewServer()
	postv1.RegisterPostServiceServer(postGRPC, fakePost)
	go func() { _ = postGRPC.Serve(postListener) }()
	t.Cleanup(postGRPC.Stop)
	conn, err := grpc.DialContext(ctx, "bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return postListener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial fake Post Service: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	server := New(
		candidate.NewRedis(redisClient),
		hydrator.NewRedisPost(redisClient, postv1.NewPostServiceClient(conn), time.Hour),
		signals.NewPostgres(pool, 30*24*time.Hour, 0),
		ranking.NewHeuristic(ranking.Weights{Recency: 1, Engagement: 2, Affinity: 1, HalfLife: 12 * time.Hour}),
		nil, 20, 20, 100, nil,
	)
	first, err := server.GetFeed(ctx, &feedv1.GetFeedRequest{UserId: viewerID, PageSize: 1})
	if err != nil {
		t.Fatalf("first GetFeed: %v", err)
	}
	if len(first.GetItems()) != 1 || first.GetItems()[0].GetPostId() != normalPostID || first.GetNextPageToken() == "" {
		t.Fatalf("first page = %+v; engagement should rank the older normal post first", first)
	}
	second, err := server.GetFeed(ctx, &feedv1.GetFeedRequest{
		UserId: viewerID, PageSize: 1, PageToken: first.GetNextPageToken(),
	})
	if err != nil {
		t.Fatalf("second GetFeed: %v", err)
	}
	if len(second.GetItems()) != 1 || second.GetItems()[0].GetPostId() != celebrityPostID {
		t.Fatalf("second page = %+v; want followed celebrity post", second)
	}
	if first.GetItems()[0].GetPostId() == second.GetItems()[0].GetPostId() {
		t.Fatal("pagination returned a duplicate")
	}
	fakePost.mu.Lock()
	defer fakePost.mu.Unlock()
	if len(fakePost.requests) != 1 || len(fakePost.requests[0]) != 1 || fakePost.requests[0][0] != celebrityPostID {
		t.Fatalf("Post Service requests = %v, want one batched request for one cache miss", fakePost.requests)
	}
	if redisClient.Exists(ctx, "post:"+strconv.FormatInt(celebrityPostID, 10)).Val() == 0 {
		t.Fatal("cache miss was not backfilled")
	}
}
