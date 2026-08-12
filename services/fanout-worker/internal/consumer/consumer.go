package consumer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/events"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Processor interface {
	Process(context.Context, string, []byte) error
}

type Publisher interface {
	PublishDLQ(context.Context, *kgo.Record, string, error, int) error
}

type RecordHandler struct {
	processor  Processor
	publisher  Publisher
	maxRetries int
	backoff    time.Duration
	sleep      func(context.Context, time.Duration) error
}

func NewRecordHandler(processor Processor, publisher Publisher, maxRetries int, backoff time.Duration) *RecordHandler {
	return &RecordHandler{
		processor: processor, publisher: publisher, maxRetries: maxRetries, backoff: backoff,
		sleep: sleepContext,
	}
}

// Handle returns nil only when the caller may commit the record: either processing
// succeeded, or the original record was durably published to its DLQ.
func (h *RecordHandler) Handle(ctx context.Context, record *kgo.Record) error {
	attempts := 0
	var processErr error
	for {
		attempts++
		processErr = h.processor.Process(ctx, record.Topic, record.Value)
		if processErr == nil {
			return nil
		}
		if errors.Is(processErr, events.ErrPermanent) || attempts > h.maxRetries {
			break
		}
		if err := h.sleep(ctx, h.backoff*time.Duration(attempts)); err != nil {
			return err
		}
	}
	if err := h.publisher.PublishDLQ(ctx, record, dlqTopic(record.Topic), processErr, attempts); err != nil {
		return fmt.Errorf("publish failed record to DLQ: %w", err)
	}
	return nil
}

type Runner struct {
	source  RecordSource
	handler *RecordHandler
}

type RecordSource interface {
	Poll(context.Context) (*kgo.Record, error)
	Commit(context.Context, *kgo.Record) error
	AllowRebalance()
}

type KafkaRecordSource struct {
	client *kgo.Client
}

func NewKafkaRecordSource(client *kgo.Client) *KafkaRecordSource {
	return &KafkaRecordSource{client: client}
}

func (s *KafkaRecordSource) Poll(ctx context.Context) (*kgo.Record, error) {
	fetches := s.client.PollRecords(ctx, 1)
	if err := fetches.Err(); err != nil {
		return nil, err
	}
	records := fetches.Records()
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

func (s *KafkaRecordSource) Commit(ctx context.Context, record *kgo.Record) error {
	return s.client.CommitRecords(ctx, record)
}

func (s *KafkaRecordSource) AllowRebalance() {
	s.client.AllowRebalance()
}

func NewRunner(source RecordSource, handler *RecordHandler) *Runner {
	return &Runner{source: source, handler: handler}
}

func (r *Runner) Run(ctx context.Context) error {
	for {
		record, err := r.source.Poll(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("poll Kafka: %w", err)
		}
		if record == nil {
			continue
		}
		if err := r.handler.Handle(ctx, record); err != nil {
			return err
		}
		// Polling one record at a time and committing synchronously ensures a later
		// offset can never be committed ahead of a failed record in the partition.
		if err := r.source.Commit(ctx, record); err != nil {
			return fmt.Errorf("commit Kafka record %s[%d]@%d: %w",
				record.Topic, record.Partition, record.Offset, err)
		}
		r.source.AllowRebalance()
	}
}

type KafkaPublisher struct {
	client         *kgo.Client
	postTopic      string
	followTopic    string
	postDLQTopic   string
	followDLQTopic string
}

func NewKafkaPublisher(client *kgo.Client, postTopic, followTopic, postDLQTopic, followDLQTopic string) *KafkaPublisher {
	return &KafkaPublisher{
		client: client, postTopic: postTopic, followTopic: followTopic,
		postDLQTopic: postDLQTopic, followDLQTopic: followDLQTopic,
	}
}

func (p *KafkaPublisher) PublishDLQ(ctx context.Context, original *kgo.Record, _ string, processErr error, attempts int) error {
	topic := p.postDLQTopic
	if original.Topic == p.followTopic {
		topic = p.followDLQTopic
	} else if original.Topic != p.postTopic {
		return fmt.Errorf("no DLQ configured for topic %q", original.Topic)
	}
	record := newDLQRecord(topic, original, processErr, attempts, time.Now())
	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce to %s: %w", topic, err)
	}
	return nil
}

func newDLQRecord(topic string, original *kgo.Record, processErr error, attempts int, failedAt time.Time) *kgo.Record {
	headers := append([]kgo.RecordHeader(nil), original.Headers...)
	headers = append(headers,
		kgo.RecordHeader{Key: "x-original-topic", Value: []byte(original.Topic)},
		kgo.RecordHeader{Key: "x-original-partition", Value: []byte(strconv.Itoa(int(original.Partition)))},
		kgo.RecordHeader{Key: "x-original-offset", Value: []byte(strconv.FormatInt(original.Offset, 10))},
		kgo.RecordHeader{Key: "x-failure-error", Value: []byte(processErr.Error())},
		kgo.RecordHeader{Key: "x-failure-attempts", Value: []byte(strconv.Itoa(attempts))},
		kgo.RecordHeader{Key: "x-failed-at-unix-ms", Value: []byte(strconv.FormatInt(failedAt.UnixMilli(), 10))},
	)
	return &kgo.Record{
		Topic: topic, Key: append([]byte(nil), original.Key...),
		Value: append([]byte(nil), original.Value...), Headers: headers,
	}
}

func dlqTopic(topic string) string {
	return topic + ".dlq"
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
