// Package feedserver implements the gRPC FeedService defined in proto/feed.proto.
//
// Phase 0 only wires up the server skeleton so the service builds, links against the
// generated proto stubs, and has something for CI to compile/test. The real implementation
// (Redis timeline read, celebrity fanout-on-read merge, cache-miss hydration, ranking,
// pagination) is added in Phase 6. See IMPLEMENTATION_PLAN.md §5.4.
package feedserver

import (
	"context"
	"log/slog"
	"sort"

	feedv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/feed/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/cursor"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CandidateStore interface {
	Load(context.Context, int64, int) (feed.CandidateSet, error)
}

type Hydrator interface {
	Hydrate(context.Context, []int64) (map[int64]feed.Post, error)
}

type SignalProvider interface {
	Load(context.Context, int64, []feed.Post) (map[int64]feed.Signal, error)
}

type Ranker interface {
	Rank([]feed.Post, map[int64]feed.Signal) []feed.RankedPost
}

// Server implements feedv1.FeedServiceServer.
type Server struct {
	feedv1.UnimplementedFeedServiceServer
	candidates      CandidateStore
	hydrator        Hydrator
	signals         SignalProvider
	ranker          Ranker
	candidatePool   int
	defaultPageSize int32
	maxPageSize     int32
	logger          *slog.Logger
}

func New(
	candidates CandidateStore,
	hydrator Hydrator,
	signals SignalProvider,
	ranker Ranker,
	candidatePool int,
	defaultPageSize, maxPageSize int32,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		candidates: candidates, hydrator: hydrator, signals: signals, ranker: ranker,
		candidatePool: candidatePool, defaultPageSize: defaultPageSize,
		maxPageSize: maxPageSize, logger: logger,
	}
}

func (s *Server) GetFeed(ctx context.Context, req *feedv1.GetFeedRequest) (*feedv1.GetFeedResponse, error) {
	if req == nil || req.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}
	pageSize := req.GetPageSize()
	if pageSize == 0 {
		pageSize = s.defaultPageSize
	}
	if pageSize < 1 || pageSize > s.maxPageSize {
		return nil, status.Errorf(codes.InvalidArgument, "page_size must be between 1 and %d", s.maxPageSize)
	}
	var pageCursor *cursor.Cursor
	if req.GetPageToken() != "" {
		decoded, err := cursor.Decode(req.GetPageToken())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		pageCursor = &decoded
	}

	candidates, err := s.candidates.Load(ctx, req.GetUserId(), s.candidatePool)
	if err != nil {
		s.logger.ErrorContext(ctx, "candidate dependency failed", "user_id", req.GetUserId(), "error", err)
		return nil, status.Error(codes.Unavailable, "candidate store unavailable")
	}
	allIDs := mergeIDs(candidates.NormalIDs, candidates.CelebrityIDs)
	hydrated, err := s.hydrator.Hydrate(ctx, allIDs)
	if err != nil {
		s.logger.ErrorContext(ctx, "post hydration dependency failed", "user_id", req.GetUserId(), "error", err)
		return nil, status.Error(codes.Unavailable, "post hydration unavailable")
	}
	posts := selectPosts(candidates, hydrated, s.candidatePool)
	loadedSignals, err := s.signals.Load(ctx, req.GetUserId(), posts)
	if err != nil {
		s.logger.ErrorContext(ctx, "ranking signal database failed", "user_id", req.GetUserId(), "error", err)
		return nil, status.Error(codes.Internal, "ranking signals unavailable")
	}
	ranked := s.ranker.Rank(posts, loadedSignals)
	start := pageStart(ranked, pageCursor)
	end := start + int(pageSize)
	if end > len(ranked) {
		end = len(ranked)
	}

	response := &feedv1.GetFeedResponse{Items: make([]*feedv1.FeedItem, 0, end-start)}
	for _, item := range ranked[start:end] {
		response.Items = append(response.Items, &feedv1.FeedItem{
			PostId: item.ID, AuthorId: item.AuthorID, Content: item.Content,
			MediaUrl: item.MediaURL, CreatedAtUnixMs: item.CreatedAt.UnixMilli(),
			RankScore: item.Score,
		})
	}
	if end < len(ranked) && end > start {
		last := ranked[end-1]
		response.NextPageToken, err = cursor.Encode(last.Score, last.CreatedAt.UnixMilli(), last.ID)
		if err != nil {
			s.logger.ErrorContext(ctx, "encode page cursor", "error", err)
			return nil, status.Error(codes.Internal, "encode pagination cursor")
		}
	}
	return response, nil
}

func mergeIDs(normal, celebrity []int64) []int64 {
	seen := make(map[int64]struct{}, len(normal)+len(celebrity))
	result := make([]int64, 0, len(normal)+len(celebrity))
	for _, ids := range [][]int64{normal, celebrity} {
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}

func selectPosts(candidates feed.CandidateSet, hydrated map[int64]feed.Post, limit int) []feed.Post {
	seen := make(map[int64]struct{}, len(hydrated))
	posts := make([]feed.Post, 0, len(hydrated))
	for _, id := range candidates.NormalIDs {
		if post, ok := hydrated[id]; ok {
			posts = append(posts, post)
			seen[id] = struct{}{}
		}
	}
	for _, id := range candidates.CelebrityIDs {
		post, ok := hydrated[id]
		if !ok {
			continue
		}
		if _, follows := candidates.FollowedCelebrities[post.AuthorID]; !follows {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		posts = append(posts, post)
		seen[id] = struct{}{}
	}
	sort.Slice(posts, func(i, j int) bool {
		if !posts[i].CreatedAt.Equal(posts[j].CreatedAt) {
			return posts[i].CreatedAt.After(posts[j].CreatedAt)
		}
		return posts[i].ID > posts[j].ID
	})
	if len(posts) > limit {
		posts = posts[:limit]
	}
	return posts
}

func pageStart(ranked []feed.RankedPost, value *cursor.Cursor) int {
	if value == nil {
		return 0
	}
	for i, item := range ranked {
		if item.ID == value.PostID && item.CreatedAt.UnixMilli() == value.CreatedAtUnixMs {
			return i + 1
		}
	}
	for i, item := range ranked {
		if item.Score < value.Score ||
			(item.Score == value.Score && item.CreatedAt.UnixMilli() < value.CreatedAtUnixMs) ||
			(item.Score == value.Score && item.CreatedAt.UnixMilli() == value.CreatedAtUnixMs && item.ID < value.PostID) {
			return i
		}
	}
	return len(ranked)
}
