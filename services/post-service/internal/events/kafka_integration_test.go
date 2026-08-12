package events

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestKafkaIntegrationPublishesPostCreated(t *testing.T) {
	brokersValue := os.Getenv("POST_SERVICE_INTEGRATION_KAFKA_BROKERS")
	if brokersValue == "" {
		t.Skip("set POST_SERVICE_INTEGRATION_KAFKA_BROKERS to run Kafka integration tests")
	}
	topic := os.Getenv("POST_SERVICE_INTEGRATION_KAFKA_TOPIC")
	if topic == "" {
		topic = "post-events"
	}
	brokers := strings.Split(brokersValue, ",")
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup("post-service-integration-"+time.Now().Format("20060102150405.000000000")),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := consumer.Ping(ctx); err != nil {
		t.Fatalf("consumer ping: %v", err)
	}
	postID := time.Now().UnixNano()
	if err := NewKafka(producer, topic).PublishCreated(ctx, post.Post{
		ID: postID, AuthorID: 42, Content: "integration", CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("PublishCreated() error = %v", err)
	}

	for {
		fetches := consumer.PollFetches(ctx)
		if err := fetches.Err(); err != nil {
			t.Fatalf("consume event: %v", err)
		}
		var found bool
		fetches.EachRecord(func(record *kgo.Record) {
			var body struct {
				EventType string `json:"eventType"`
				PostID    int64  `json:"postId"`
			}
			if json.Unmarshal(record.Value, &body) == nil && body.PostID == postID && body.EventType == "PostCreated" {
				found = true
			}
		})
		if found {
			return
		}
	}
}
