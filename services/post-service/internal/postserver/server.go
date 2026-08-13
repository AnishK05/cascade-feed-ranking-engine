// Package postserver implements the gRPC PostService defined in proto/post.proto.
package postserver

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/observability"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/repository"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxContentLength  = 5000
	maxMediaURLLength = 2048
	sideEffectTimeout = 5 * time.Second
)

type Repository interface {
	Create(context.Context, int64, string, string) (post.Post, error)
	GetByIDs(context.Context, []int64) (map[int64]post.Post, error)
	Delete(context.Context, int64, int64) (int64, time.Time, error)
}

type Cache interface {
	Set(context.Context, post.Post) error
	SetMany(context.Context, []post.Post) error
	GetMany(context.Context, []int64) (map[int64]post.Post, error)
	DeleteAndTombstone(context.Context, int64) error
}

type EventPublisher interface {
	PublishCreated(context.Context, post.Post) error
	PublishDeleted(context.Context, int64, int64, time.Time) error
}

type Server struct {
	postv1.UnimplementedPostServiceServer
	repository Repository
	cache      Cache
	publisher  EventPublisher
	logger     *slog.Logger
}

func New(repository Repository, cache Cache, publisher EventPublisher, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{repository: repository, cache: cache, publisher: publisher, logger: logger}
}

func (s *Server) CreatePost(ctx context.Context, req *postv1.CreatePostRequest) (*postv1.CreatePostResponse, error) {
	if err := validateCreate(req); err != nil {
		return nil, err
	}
	p, err := s.repository.Create(ctx, req.GetAuthorId(), strings.TrimSpace(req.GetContent()), strings.TrimSpace(req.GetMediaUrl()))
	if err != nil {
		return nil, createError(err)
	}

	// The database commit has succeeded. Cache and Kafka are deliberately best-effort: their
	// failure is logged and the successful RPC is returned because rolling back is no longer
	// possible. A future outbox/reconciliation process can repair missed delivery.
	sideEffectBase := context.WithoutCancel(ctx)
	cacheCtx, cancelCache := context.WithTimeout(sideEffectBase, sideEffectTimeout)
	if err := s.cache.Set(cacheCtx, p); err != nil {
		s.logger.Error("post committed but cache write failed", "post_id", p.ID, "error", err)
	}
	cancelCache()

	// Give Kafka its own timeout. A slow Redis write must not consume the publisher's entire
	// post-commit side-effect budget and prevent us from even attempting event delivery.
	publishCtx, cancelPublish := context.WithTimeout(sideEffectBase, sideEffectTimeout)
	defer cancelPublish()
	if err := s.publisher.PublishCreated(publishCtx, p); err != nil {
		s.logger.Error("post committed but PostCreated publish failed", "post_id", p.ID, "error", err)
	}

	return &postv1.CreatePostResponse{
		PostId:          p.ID,
		CreatedAtUnixMs: p.CreatedAt.UnixMilli(),
	}, nil
}

func (s *Server) GetPosts(ctx context.Context, req *postv1.GetPostsRequest) (*postv1.GetPostsResponse, error) {
	if req == nil || len(req.GetPostIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "post_ids must contain at least one ID")
	}
	for _, id := range req.GetPostIds() {
		if id <= 0 {
			return nil, status.Error(codes.InvalidArgument, "post_ids must contain only positive IDs")
		}
	}

	uniqueIDs := unique(req.GetPostIds())
	cached, err := s.cache.GetMany(ctx, uniqueIDs)
	if err != nil {
		s.logger.Warn("post cache read failed; falling back to PostgreSQL", "error", err)
		cached = map[int64]post.Post{}
	}

	missing := make([]int64, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		if _, ok := cached[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		hydrated, err := s.repository.GetByIDs(ctx, missing)
		if err != nil {
			return nil, rpcError(err, "get posts")
		}
		observability.RecordPostgresQuery("get_posts")
		toCache := make([]post.Post, 0, len(hydrated))
		for id, p := range hydrated {
			cached[id] = p
			toCache = append(toCache, p)
		}
		if err := s.cache.SetMany(ctx, toCache); err != nil {
			s.logger.Warn("post cache backfill failed", "error", err)
		}
	}

	response := &postv1.GetPostsResponse{Posts: make([]*postv1.Post, 0, len(req.GetPostIds()))}
	for _, id := range req.GetPostIds() {
		if p, ok := cached[id]; ok {
			response.Posts = append(response.Posts, toProto(p))
		}
	}
	return response, nil
}

func (s *Server) DeletePost(ctx context.Context, req *postv1.DeletePostRequest) (*postv1.DeletePostResponse, error) {
	if req == nil || req.GetPostId() <= 0 || req.GetRequestingUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "post_id and requesting_user_id must be positive")
	}
	authorID, deletedAt, err := s.repository.Delete(ctx, req.GetPostId(), req.GetRequestingUserId())
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, status.Error(codes.NotFound, "post not found")
		case errors.Is(err, repository.ErrForbidden):
			return nil, status.Error(codes.PermissionDenied, "requesting user is not the post author")
		default:
			return nil, rpcError(err, "delete post")
		}
	}

	sideEffectBase := context.WithoutCancel(ctx)
	cacheCtx, cancelCache := context.WithTimeout(sideEffectBase, sideEffectTimeout)
	if err := s.cache.DeleteAndTombstone(cacheCtx, req.GetPostId()); err != nil {
		s.logger.Error("post deleted but cache invalidation failed", "post_id", req.GetPostId(), "error", err)
	}
	cancelCache()

	publishCtx, cancelPublish := context.WithTimeout(sideEffectBase, sideEffectTimeout)
	defer cancelPublish()
	if err := s.publisher.PublishDeleted(publishCtx, req.GetPostId(), authorID, deletedAt); err != nil {
		s.logger.Error("post deleted but PostDeleted publish failed", "post_id", req.GetPostId(), "error", err)
	}
	return &postv1.DeletePostResponse{Ok: true}, nil
}

func validateCreate(req *postv1.CreatePostRequest) error {
	if req == nil || req.GetAuthorId() <= 0 {
		return status.Error(codes.InvalidArgument, "author_id must be positive")
	}
	content := strings.TrimSpace(req.GetContent())
	if content == "" {
		return status.Error(codes.InvalidArgument, "content must not be empty")
	}
	if len([]rune(content)) > maxContentLength {
		return status.Errorf(codes.InvalidArgument, "content must be at most %d characters", maxContentLength)
	}
	mediaURL := strings.TrimSpace(req.GetMediaUrl())
	if len(mediaURL) > maxMediaURLLength {
		return status.Errorf(codes.InvalidArgument, "media_url must be at most %d characters", maxMediaURLLength)
	}
	if mediaURL != "" {
		parsed, err := url.ParseRequestURI(mediaURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return status.Error(codes.InvalidArgument, "media_url must be an absolute HTTP(S) URL")
		}
	}
	return nil
}

func createError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return status.Error(codes.NotFound, "author not found")
	}
	return rpcError(err, "create post")
}

func rpcError(err error, operation string) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, operation+" canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, operation+" deadline exceeded")
	default:
		return status.Error(codes.Internal, operation+" failed")
	}
}

func unique(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}

func toProto(p post.Post) *postv1.Post {
	return &postv1.Post{
		Id:              p.ID,
		AuthorId:        p.AuthorID,
		Content:         p.Content,
		MediaUrl:        p.MediaURL,
		CreatedAtUnixMs: p.CreatedAt.UnixMilli(),
	}
}
