package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Kafka struct {
	producer *kgo.Client
	topic    string
}

func NewKafka(producer *kgo.Client, topic string) *Kafka {
	return &Kafka{producer: producer, topic: topic}
}

type postCreated struct {
	EventType      string `json:"eventType"`
	PostID         int64  `json:"postId"`
	AuthorID       int64  `json:"authorId"`
	CreatedAtUnixM int64  `json:"createdAtUnixMs"`
}

type postDeleted struct {
	EventType      string `json:"eventType"`
	PostID         int64  `json:"postId"`
	AuthorID       int64  `json:"authorId"`
	DeletedAtUnixM int64  `json:"deletedAtUnixMs"`
}

func (k *Kafka) PublishCreated(ctx context.Context, p post.Post) error {
	return k.publish(ctx, p.AuthorID, postCreated{
		EventType:      "PostCreated",
		PostID:         p.ID,
		AuthorID:       p.AuthorID,
		CreatedAtUnixM: p.CreatedAt.UnixMilli(),
	})
}

func (k *Kafka) PublishDeleted(ctx context.Context, postID, authorID int64, deletedAt time.Time) error {
	return k.publish(ctx, authorID, postDeleted{
		EventType:      "PostDeleted",
		PostID:         postID,
		AuthorID:       authorID,
		DeletedAtUnixM: deletedAt.UnixMilli(),
	})
}

func (k *Kafka) publish(ctx context.Context, authorID int64, event any) error {
	record, err := newRecord(k.topic, authorID, event)
	if err != nil {
		return err
	}
	if err := k.producer.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("publish post event: %w", err)
	}
	return nil
}

func newRecord(topic string, authorID int64, event any) (*kgo.Record, error) {
	value, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal post event: %w", err)
	}
	return &kgo.Record{
		Topic: topic,
		Key:   []byte(strconv.FormatInt(authorID, 10)),
		Value: value,
	}, nil
}
