package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/events"
	"github.com/twmb/franz-go/pkg/kgo"
)

type fakeProcessor struct {
	errors []error
	calls  int
}

func (f *fakeProcessor) Process(context.Context, string, []byte) error {
	index := f.calls
	f.calls++
	if index < len(f.errors) {
		return f.errors[index]
	}
	return nil
}

type fakePublisher struct {
	calls    int
	topic    string
	payload  []byte
	attempts int
	err      error
}

func (f *fakePublisher) PublishDLQ(_ context.Context, record *kgo.Record, topic string, _ error, attempts int) error {
	f.calls++
	f.topic, f.payload, f.attempts = topic, append([]byte(nil), record.Value...), attempts
	return f.err
}

func noSleep(context.Context, time.Duration) error { return nil }

type fakeSource struct {
	record     *kgo.Record
	polls      int
	commits    int
	rebalances int
}

func (f *fakeSource) Poll(context.Context) (*kgo.Record, error) {
	f.polls++
	if f.polls == 1 {
		return f.record, nil
	}
	return nil, context.Canceled
}
func (f *fakeSource) Commit(context.Context, *kgo.Record) error {
	f.commits++
	return nil
}
func (f *fakeSource) AllowRebalance() { f.rebalances++ }

func TestRecordHandlerRetriesThenAllowsCommit(t *testing.T) {
	processor := &fakeProcessor{errors: []error{errors.New("redis unavailable"), errors.New("redis unavailable")}}
	publisher := &fakePublisher{}
	handler := NewRecordHandler(processor, publisher, 3, time.Millisecond)
	handler.sleep = noSleep
	if err := handler.Handle(context.Background(), &kgo.Record{Topic: "post-events", Value: []byte("body")}); err != nil {
		t.Fatal(err)
	}
	if processor.calls != 3 || publisher.calls != 0 {
		t.Fatalf("processor calls = %d, publisher calls = %d", processor.calls, publisher.calls)
	}
}

func TestRecordHandlerExhaustionPublishesOriginalToDLQThenAllowsCommit(t *testing.T) {
	transient := errors.New("postgres unavailable")
	processor := &fakeProcessor{errors: []error{transient, transient, transient}}
	publisher := &fakePublisher{}
	handler := NewRecordHandler(processor, publisher, 2, time.Millisecond)
	handler.sleep = noSleep
	record := &kgo.Record{Topic: "follow-events", Value: []byte(`{"original":true}`)}
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if processor.calls != 3 || publisher.calls != 1 || publisher.topic != "follow-events.dlq" ||
		string(publisher.payload) != string(record.Value) || publisher.attempts != 3 {
		t.Fatalf("unexpected retry/DLQ state: processor=%d publisher=%+v", processor.calls, publisher)
	}
}

func TestRecordHandlerPermanentErrorGoesDirectlyToDLQ(t *testing.T) {
	permanent := errors.Join(events.ErrPermanent, errors.New("malformed JSON"))
	processor := &fakeProcessor{errors: []error{permanent}}
	publisher := &fakePublisher{}
	handler := NewRecordHandler(processor, publisher, 10, time.Hour)
	handler.sleep = func(context.Context, time.Duration) error {
		t.Fatal("permanent errors must not back off")
		return nil
	}
	if err := handler.Handle(context.Background(), &kgo.Record{Topic: "post-events", Value: []byte("{")}); err != nil {
		t.Fatal(err)
	}
	if processor.calls != 1 || publisher.calls != 1 || publisher.attempts != 1 {
		t.Fatalf("processor calls = %d, publisher = %+v", processor.calls, publisher)
	}
}

func TestRecordHandlerDoesNotAllowCommitWhenDLQPublishFails(t *testing.T) {
	processor := &fakeProcessor{errors: []error{errors.Join(events.ErrPermanent, errors.New("unknown event"))}}
	publisher := &fakePublisher{err: errors.New("kafka unavailable")}
	handler := NewRecordHandler(processor, publisher, 2, time.Millisecond)
	if err := handler.Handle(context.Background(), &kgo.Record{Topic: "post-events"}); err == nil {
		t.Fatal("Handle returned nil despite failed DLQ publication; caller would incorrectly commit")
	}
}

func TestRunnerCommitsOnlyAfterHandledRecord(t *testing.T) {
	t.Run("successful processing commits exactly once", func(t *testing.T) {
		source := &fakeSource{record: &kgo.Record{Topic: "post-events", Offset: 4}}
		handler := NewRecordHandler(&fakeProcessor{}, &fakePublisher{}, 1, time.Millisecond)
		if err := NewRunner(source, handler).Run(context.Background()); err != nil {
			t.Fatal(err)
		}
		if source.commits != 1 || source.rebalances != 1 {
			t.Fatalf("commits = %d, rebalances = %d", source.commits, source.rebalances)
		}
	})

	t.Run("failed DLQ publication does not commit", func(t *testing.T) {
		permanent := errors.Join(events.ErrPermanent, errors.New("bad event"))
		source := &fakeSource{record: &kgo.Record{Topic: "post-events", Offset: 5}}
		handler := NewRecordHandler(
			&fakeProcessor{errors: []error{permanent}},
			&fakePublisher{err: errors.New("DLQ unavailable")},
			1, time.Millisecond,
		)
		if err := NewRunner(source, handler).Run(context.Background()); err == nil {
			t.Fatal("Run returned nil")
		}
		if source.commits != 0 {
			t.Fatalf("commits = %d, want 0", source.commits)
		}
	})
}

func TestDLQRecordPreservesPayloadKeyAndFailureMetadata(t *testing.T) {
	original := &kgo.Record{
		Topic: "post-events", Partition: 2, Offset: 9,
		Key: []byte("42"), Value: []byte(`{"bad":true}`),
		Headers: []kgo.RecordHeader{{Key: "trace-id", Value: []byte("abc")}},
	}
	record := newDLQRecord("custom-post-dlq", original, errors.New("malformed"), 1, time.UnixMilli(1234))
	if record.Topic != "custom-post-dlq" || string(record.Key) != "42" ||
		string(record.Value) != string(original.Value) {
		t.Fatalf("DLQ record did not preserve identity/payload: %+v", record)
	}
	headers := make(map[string]string, len(record.Headers))
	for _, header := range record.Headers {
		headers[header.Key] = string(header.Value)
	}
	for key, want := range map[string]string{
		"trace-id": "abc", "x-original-topic": "post-events",
		"x-original-partition": "2", "x-original-offset": "9",
		"x-failure-error": "malformed", "x-failure-attempts": "1",
		"x-failed-at-unix-ms": "1234",
	} {
		if headers[key] != want {
			t.Errorf("header %s = %q, want %q", key, headers[key], want)
		}
	}
}
