package fanout

import (
	"context"
	"errors"
	"testing"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/events"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
)

type fakeRepository struct {
	count     int64
	followers []int64
	posts     []repository.Post
	countErr  error
}

func (f *fakeRepository) FollowerCount(context.Context, int64) (int64, error) {
	return f.count, f.countErr
}
func (f *fakeRepository) FollowerIDs(context.Context, int64) ([]int64, error) {
	return f.followers, nil
}
func (f *fakeRepository) RecentPosts(context.Context, int64, int64) ([]repository.Post, error) {
	return f.posts, nil
}

type fakeTimeline struct {
	cachedCount int64
	cacheHit    bool
	fanout      []int64
	fanoutPost  repository.Post
	celebrity   *repository.Post
	tombstone   int64
	backfilled  []repository.Post
	backfillFor int64
	addFollow   [2]int64
	remove      [2]int64
	invalidated []int64
}

func (f *fakeTimeline) CachedFollowerCount(context.Context, int64) (int64, bool, error) {
	return f.cachedCount, f.cacheHit, nil
}
func (f *fakeTimeline) CacheFollowerCount(_ context.Context, _ int64, count int64) error {
	f.cachedCount, f.cacheHit = count, true
	return nil
}
func (f *fakeTimeline) InvalidateFollowerCount(_ context.Context, id int64) error {
	f.invalidated = append(f.invalidated, id)
	f.cacheHit = false
	return nil
}
func (f *fakeTimeline) FanoutPost(_ context.Context, ids []int64, post repository.Post, _ int64, _ int) error {
	f.fanout, f.fanoutPost = ids, post
	return nil
}
func (f *fakeTimeline) AddCelebrityPost(_ context.Context, post repository.Post, _ int64) error {
	f.celebrity = &post
	return nil
}
func (f *fakeTimeline) AddTombstone(_ context.Context, id int64) error {
	f.tombstone = id
	return nil
}
func (f *fakeTimeline) Backfill(_ context.Context, id int64, posts []repository.Post, _ int64) error {
	f.backfillFor, f.backfilled = id, posts
	return nil
}
func (f *fakeTimeline) AddCelebrityFollow(_ context.Context, follower, followee int64) error {
	f.addFollow = [2]int64{follower, followee}
	return nil
}
func (f *fakeTimeline) RemoveCelebrityFollow(_ context.Context, follower, followee int64) error {
	f.remove = [2]int64{follower, followee}
	return nil
}

func newTestProcessor(repo Repository, timeline Timeline) *Processor {
	return NewProcessor(repo, timeline, Settings{
		PostTopic: "post-events", FollowTopic: "follow-events",
		CelebrityFollowerThreshold: 10, MaxTimelineLen: 100,
		BackfillCount: 3, FanoutBatchSize: 2,
	})
}

func TestPostCreatedNormalAndCelebrity(t *testing.T) {
	t.Run("normal account fans out to every follower", func(t *testing.T) {
		repo := &fakeRepository{count: 2, followers: []int64{10, 11}}
		timelines := &fakeTimeline{}
		err := newTestProcessor(repo, timelines).Process(context.Background(), "post-events",
			[]byte(`{"eventType":"PostCreated","postId":7,"authorId":4,"createdAtUnixMs":123}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(timelines.fanout) != 2 || timelines.fanoutPost.ID != 7 || timelines.celebrity != nil {
			t.Fatalf("unexpected normal fanout: %+v", timelines)
		}
	})

	t.Run("celebrity writes global zset only", func(t *testing.T) {
		repo := &fakeRepository{count: 10, followers: []int64{10, 11}}
		timelines := &fakeTimeline{}
		err := newTestProcessor(repo, timelines).Process(context.Background(), "post-events",
			[]byte(`{"eventType":"PostCreated","postId":8,"authorId":5,"createdAtUnixMs":124}`))
		if err != nil {
			t.Fatal(err)
		}
		if timelines.celebrity == nil || timelines.celebrity.ID != 8 || timelines.fanout != nil {
			t.Fatalf("unexpected celebrity fanout: %+v", timelines)
		}
	})

	t.Run("cached count avoids repository", func(t *testing.T) {
		repo := &fakeRepository{countErr: errors.New("must not query"), followers: []int64{10}}
		timelines := &fakeTimeline{cachedCount: 1, cacheHit: true}
		if err := newTestProcessor(repo, timelines).Process(context.Background(), "post-events",
			[]byte(`{"eventType":"PostCreated","postId":9,"authorId":5,"createdAtUnixMs":125}`)); err != nil {
			t.Fatal(err)
		}
	})
}

func TestPostDeletedAddsIdempotentTombstone(t *testing.T) {
	timelines := &fakeTimeline{}
	payload := []byte(`{"eventType":"PostDeleted","postId":7,"authorId":4,"deletedAtUnixMs":123}`)
	processor := newTestProcessor(&fakeRepository{}, timelines)
	if err := processor.Process(context.Background(), "post-events", payload); err != nil {
		t.Fatal(err)
	}
	if err := processor.Process(context.Background(), "post-events", payload); err != nil {
		t.Fatal(err)
	}
	if timelines.tombstone != 7 {
		t.Fatalf("tombstone = %d", timelines.tombstone)
	}
}

func TestFollowCreatedBackfillsNormalOrTracksCelebrity(t *testing.T) {
	posts := []repository.Post{{ID: 1, CreatedAtUnixMs: 10}, {ID: 2, CreatedAtUnixMs: 9}}
	t.Run("normal", func(t *testing.T) {
		timelines := &fakeTimeline{cacheHit: true, cachedCount: 99}
		repo := &fakeRepository{count: 9, posts: posts}
		err := newTestProcessor(repo, timelines).Process(context.Background(), "follow-events",
			[]byte(`{"eventType":"FollowCreated","followerId":6,"followeeId":4,"createdAtUnixMs":123}`))
		if err != nil {
			t.Fatal(err)
		}
		if timelines.backfillFor != 6 || len(timelines.backfilled) != 2 || timelines.addFollow != [2]int64{} {
			t.Fatalf("unexpected backfill: %+v", timelines)
		}
	})
	t.Run("celebrity", func(t *testing.T) {
		timelines := &fakeTimeline{}
		repo := &fakeRepository{count: 10, posts: posts}
		err := newTestProcessor(repo, timelines).Process(context.Background(), "follow-events",
			[]byte(`{"eventType":"FollowCreated","followerId":6,"followeeId":4,"createdAtUnixMs":123}`))
		if err != nil {
			t.Fatal(err)
		}
		if timelines.addFollow != [2]int64{6, 4} || timelines.backfilled != nil {
			t.Fatalf("unexpected celebrity follow: %+v", timelines)
		}
	})
}

func TestFollowDeletedRemovesCelebrityMarkerAndInvalidatesCount(t *testing.T) {
	timelines := &fakeTimeline{}
	err := newTestProcessor(&fakeRepository{}, timelines).Process(context.Background(), "follow-events",
		[]byte(`{"eventType":"FollowDeleted","followerId":6,"followeeId":4,"deletedAtUnixMs":123}`))
	if err != nil {
		t.Fatal(err)
	}
	if timelines.remove != [2]int64{6, 4} || len(timelines.invalidated) != 1 || timelines.invalidated[0] != 4 {
		t.Fatalf("unexpected follow delete: %+v", timelines)
	}
	if timelines.fanout != nil {
		t.Fatal("follow deletion must not remove historical normal posts")
	}
}

func TestMalformedAndUnknownEventsArePermanent(t *testing.T) {
	tests := []struct {
		topic   string
		payload string
	}{
		{"post-events", `{`},
		{"post-events", `{"eventType":"PostEdited"}`},
		{"follow-events", `{"eventType":"FollowCreated","followerId":0}`},
		{"other", `{"eventType":"PostCreated"}`},
	}
	for _, test := range tests {
		err := newTestProcessor(&fakeRepository{}, &fakeTimeline{}).
			Process(context.Background(), test.topic, []byte(test.payload))
		if !errors.Is(err, events.ErrPermanent) {
			t.Errorf("Process(%q, %q) error = %v, want permanent", test.topic, test.payload, err)
		}
	}
}
