package fanout

import (
	"context"
	"fmt"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/events"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
)

type Repository interface {
	FollowerCount(context.Context, int64) (int64, error)
	FollowerIDs(context.Context, int64) ([]int64, error)
	RecentPosts(context.Context, int64, int64) ([]repository.Post, error)
}

type Timeline interface {
	CachedFollowerCount(context.Context, int64) (int64, bool, error)
	CacheFollowerCount(context.Context, int64, int64) error
	InvalidateFollowerCount(context.Context, int64) error
	FanoutPost(context.Context, []int64, repository.Post, int64, int) error
	AddCelebrityPost(context.Context, repository.Post, int64) error
	AddTombstone(context.Context, int64) error
	Backfill(context.Context, int64, []repository.Post, int64) error
	AddCelebrityFollow(context.Context, int64, int64) error
	RemoveCelebrityFollow(context.Context, int64, int64) error
}

type Settings struct {
	PostTopic                  string
	FollowTopic                string
	CelebrityFollowerThreshold int64
	MaxTimelineLen             int64
	BackfillCount              int64
	FanoutBatchSize            int
}

type Processor struct {
	repository Repository
	timeline   Timeline
	settings   Settings
}

func NewProcessor(repository Repository, timeline Timeline, settings Settings) *Processor {
	return &Processor{repository: repository, timeline: timeline, settings: settings}
}

func (p *Processor) Process(ctx context.Context, topic string, payload []byte) error {
	switch topic {
	case p.settings.PostTopic:
		event, err := events.ParsePost(payload)
		if err != nil {
			return err
		}
		return p.processPost(ctx, event)
	case p.settings.FollowTopic:
		event, err := events.ParseFollow(payload)
		if err != nil {
			return err
		}
		return p.processFollow(ctx, event)
	default:
		return fmt.Errorf("%w: record from unconfigured topic %q", events.ErrPermanent, topic)
	}
}

func (p *Processor) processPost(ctx context.Context, event any) error {
	switch event := event.(type) {
	case events.PostCreated:
		count, err := p.followerCount(ctx, event.AuthorID)
		if err != nil {
			return err
		}
		post := repository.Post{ID: event.PostID, CreatedAtUnixMs: event.CreatedAtUnixMs}
		if !ShouldFanoutOnWrite(count, p.settings.CelebrityFollowerThreshold) {
			return p.timeline.AddCelebrityPost(ctx, post, p.settings.MaxTimelineLen)
		}
		followers, err := p.repository.FollowerIDs(ctx, event.AuthorID)
		if err != nil {
			return err
		}
		return p.timeline.FanoutPost(ctx, followers, post, p.settings.MaxTimelineLen, p.settings.FanoutBatchSize)
	case events.PostDeleted:
		return p.timeline.AddTombstone(ctx, event.PostID)
	default:
		return fmt.Errorf("%w: unsupported decoded post event %T", events.ErrPermanent, event)
	}
}

func (p *Processor) processFollow(ctx context.Context, event any) error {
	switch event := event.(type) {
	case events.FollowCreated:
		if err := p.timeline.InvalidateFollowerCount(ctx, event.FolloweeID); err != nil {
			return err
		}
		count, err := p.followerCount(ctx, event.FolloweeID)
		if err != nil {
			return err
		}
		if !ShouldFanoutOnWrite(count, p.settings.CelebrityFollowerThreshold) {
			return p.timeline.AddCelebrityFollow(ctx, event.FollowerID, event.FolloweeID)
		}
		posts, err := p.repository.RecentPosts(ctx, event.FolloweeID, p.settings.BackfillCount)
		if err != nil {
			return err
		}
		return p.timeline.Backfill(ctx, event.FollowerID, posts, p.settings.MaxTimelineLen)
	case events.FollowDeleted:
		// SREM is safe whether or not the followee is currently a celebrity. Historical
		// normal-account posts are intentionally left in the bounded timeline and age out.
		if err := p.timeline.RemoveCelebrityFollow(ctx, event.FollowerID, event.FolloweeID); err != nil {
			return err
		}
		return p.timeline.InvalidateFollowerCount(ctx, event.FolloweeID)
	default:
		return fmt.Errorf("%w: unsupported decoded follow event %T", events.ErrPermanent, event)
	}
}

func (p *Processor) followerCount(ctx context.Context, userID int64) (int64, error) {
	count, ok, err := p.timeline.CachedFollowerCount(ctx, userID)
	if err != nil {
		return 0, err
	}
	if ok {
		return count, nil
	}
	count, err = p.repository.FollowerCount(ctx, userID)
	if err != nil {
		return 0, err
	}
	if err := p.timeline.CacheFollowerCount(ctx, userID, count); err != nil {
		return 0, err
	}
	return count, nil
}
