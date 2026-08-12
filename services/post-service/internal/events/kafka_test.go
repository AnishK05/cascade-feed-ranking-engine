package events

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func TestPostEventRecords(t *testing.T) {
	tests := []struct {
		name      string
		authorID  int64
		event     any
		eventType string
		timeField string
	}{
		{
			name: "created", authorID: 42,
			event:     postCreated{EventType: "PostCreated", PostID: 7, AuthorID: 42, CreatedAtUnixM: 1234},
			eventType: "PostCreated", timeField: "createdAtUnixMs",
		},
		{
			name: "deleted", authorID: 55,
			event:     postDeleted{EventType: "PostDeleted", PostID: 8, AuthorID: 55, DeletedAtUnixM: 5678},
			eventType: "PostDeleted", timeField: "deletedAtUnixMs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := newRecord("post-events", tt.authorID, tt.event)
			if err != nil {
				t.Fatalf("newRecord() error = %v", err)
			}
			if record.Topic != "post-events" || string(record.Key) != strconv.FormatInt(tt.authorID, 10) {
				t.Fatalf("record topic/key = %q/%q", record.Topic, record.Key)
			}
			var body map[string]any
			if err := json.Unmarshal(record.Value, &body); err != nil {
				t.Fatalf("event is not JSON: %v", err)
			}
			if body["eventType"] != tt.eventType || body[tt.timeField] == nil {
				t.Fatalf("event JSON = %s", record.Value)
			}
		})
	}
}

func TestEventTimestampsUseUnixMilliseconds(t *testing.T) {
	at := time.UnixMilli(1732400000123)
	created := postCreated{CreatedAtUnixM: at.UnixMilli()}
	deleted := postDeleted{DeletedAtUnixM: at.UnixMilli()}
	if created.CreatedAtUnixM != 1732400000123 || deleted.DeletedAtUnixM != 1732400000123 {
		t.Fatalf("timestamps = %d/%d", created.CreatedAtUnixM, deleted.DeletedAtUnixM)
	}
}
