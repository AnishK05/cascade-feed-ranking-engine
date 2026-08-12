package fanout_worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/events"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/fanout"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/timeline"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestRealBackendsNormalCelebrityAndRedelivery(t *testing.T) {
	if os.Getenv("FANOUT_INTEGRATION_TEST") != "1" {
		t.Skip("set FANOUT_INTEGRATION_TEST=1 to run against real Postgres, Redis, and Kafka")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	databaseURL := envOr("DATABASE_URL", "postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable")
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:9092"), ",")

	var normalID, celebrityID, followerID, normalPostID, celebrityPostID int64
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = redisClient.Close() })
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if normalPostID != 0 || celebrityPostID != 0 {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.posts WHERE id = ANY($1)`, []int64{normalPostID, celebrityPostID})
		}
		if followerID != 0 {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.follows WHERE follower_id = $1`, followerID)
		}
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM public.users WHERE id = ANY($1)`, []int64{normalID, celebrityID, followerID})
		_ = redisClient.Del(cleanupCtx,
			fmt.Sprintf("timeline:%d", followerID),
			fmt.Sprintf("following:celebrities:%d", followerID),
			fmt.Sprintf("fanout:follower_count:%d", normalID),
			fmt.Sprintf("fanout:follower_count:%d", celebrityID),
		).Err()
		if celebrityPostID != 0 {
			_ = redisClient.ZRem(cleanupCtx, "celebrity_posts:global", celebrityPostID).Err()
		}
	})

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	postTopic := "fanout-post-integration-" + suffix
	followTopic := "fanout-follow-integration-" + suffix
	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup("fanout-integration-"+suffix),
		kgo.ConsumeTopics(postTopic, followTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(kafkaClient.Close)
	createTopics(t, ctx, kafkaClient, postTopic, followTopic)

	username := func(kind string) string { return "fanout_it_" + kind + "_" + suffix }
	if err := pool.QueryRow(ctx, `
		INSERT INTO public.users (username, display_name, follower_count)
		VALUES ($1, $2, $3) RETURNING id`, username("normal"), "Normal", 1).Scan(&normalID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO public.users (username, display_name, follower_count, is_celebrity)
		VALUES ($1, $2, $3, true) RETURNING id`, username("celebrity"), "Celebrity", 2).Scan(&celebrityID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO public.users (username, display_name)
		VALUES ($1, $2) RETURNING id`, username("follower"), "Follower").Scan(&followerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.follows (follower_id, followee_id) VALUES ($1, $2)`,
		followerID, normalID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO public.posts (author_id, content) VALUES ($1, 'normal integration post')
		RETURNING id`, normalID).Scan(&normalPostID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO public.posts (author_id, content) VALUES ($1, 'celebrity integration post')
		RETURNING id`, celebrityID).Scan(&celebrityPostID); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewPostgres(pool)
	timelines := timeline.NewRedis(redisClient, time.Minute)
	processor := fanout.NewProcessor(repo, timelines, fanout.Settings{
		PostTopic: postTopic, FollowTopic: followTopic,
		CelebrityFollowerThreshold: 2, MaxTimelineLen: 10,
		BackfillCount: 2, FanoutBatchSize: 2,
	})
	celebrityBaseline := redisClient.ZCard(ctx, "celebrity_posts:global").Val()
	now := time.Now().UnixMilli()
	payloads := []struct {
		topic string
		key   int64
		body  any
	}{
		{postTopic, normalID, events.PostCreated{
			EventType: "PostCreated", PostID: normalPostID, AuthorID: normalID, CreatedAtUnixMs: now,
		}},
		{postTopic, celebrityID, events.PostCreated{
			EventType: "PostCreated", PostID: celebrityPostID, AuthorID: celebrityID, CreatedAtUnixMs: now + 1,
		}},
		{followTopic, celebrityID, events.FollowCreated{
			EventType: "FollowCreated", FollowerID: followerID, FolloweeID: celebrityID, CreatedAtUnixMs: now + 2,
		}},
	}
	for _, item := range payloads {
		value, err := json.Marshal(item.body)
		if err != nil {
			t.Fatal(err)
		}
		record := &kgo.Record{Topic: item.topic, Key: []byte(strconv.FormatInt(item.key, 10)), Value: value}
		if err := kafkaClient.ProduceSync(ctx, record).FirstErr(); err != nil {
			t.Fatal(err)
		}
	}

	processed := 0
	for processed < len(payloads) {
		fetches := kafkaClient.PollRecords(ctx, 1)
		if err := fetches.Err(); err != nil {
			t.Fatal(err)
		}
		for _, record := range fetches.Records() {
			if err := processor.Process(ctx, record.Topic, record.Value); err != nil {
				t.Fatal(err)
			}
			// Explicitly replay every Kafka record. ZADD/SADD must keep cardinality at one.
			if err := processor.Process(ctx, record.Topic, record.Value); err != nil {
				t.Fatal(err)
			}
			processed++
		}
	}

	if got := redisClient.ZCard(ctx, fmt.Sprintf("timeline:%d", followerID)).Val(); got != 1 {
		t.Errorf("normal timeline cardinality after redelivery = %d, want 1", got)
	}
	wantCelebrityCount := min(celebrityBaseline+1, int64(10))
	if got := redisClient.ZCard(ctx, "celebrity_posts:global").Val(); got != wantCelebrityCount {
		t.Errorf("celebrity global cardinality after redelivery = %d, want %d", got, wantCelebrityCount)
	}
	if _, err := redisClient.ZScore(ctx, "celebrity_posts:global", strconv.FormatInt(celebrityPostID, 10)).Result(); err != nil {
		t.Errorf("celebrity post missing after fanout: %v", err)
	}
	if !redisClient.SIsMember(ctx, fmt.Sprintf("following:celebrities:%d", followerID),
		strconv.FormatInt(celebrityID, 10)).Val() {
		t.Error("celebrity following set does not contain followee")
	}
}

func createTopics(t *testing.T, ctx context.Context, client *kgo.Client, topics ...string) {
	t.Helper()
	request := kmsg.NewPtrCreateTopicsRequest()
	request.TimeoutMillis = 10_000
	for _, topic := range topics {
		request.Topics = append(request.Topics, kmsg.CreateTopicsRequestTopic{
			Topic: topic, NumPartitions: 1, ReplicationFactor: 1,
		})
	}
	response, err := request.RequestWith(ctx, client)
	if err != nil {
		t.Fatalf("create integration topics: %v", err)
	}
	for _, topic := range response.Topics {
		if topicErr := kerr.ErrorForCode(topic.ErrorCode); topicErr != nil && topicErr != kerr.TopicAlreadyExists {
			t.Fatalf("create integration topic %s: %v", topic.Topic, topicErr)
		}
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
