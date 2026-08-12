package postserver

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/post"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/repository"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestServerImplementsInterface(t *testing.T) {
	var _ postv1.PostServiceServer = New(&fakeRepository{}, &fakeCache{}, &fakePublisher{}, nil)
}

func TestCreatePostValidation(t *testing.T) {
	tests := []struct {
		name string
		req  *postv1.CreatePostRequest
	}{
		{"nil request", nil},
		{"invalid author", &postv1.CreatePostRequest{Content: "hello"}},
		{"blank content", &postv1.CreatePostRequest{AuthorId: 1, Content: " \t"}},
		{"content too long", &postv1.CreatePostRequest{AuthorId: 1, Content: strings.Repeat("a", maxContentLength+1)}},
		{"invalid media URL", &postv1.CreatePostRequest{AuthorId: 1, Content: "hello", MediaUrl: "relative/path"}},
		{"unsupported media scheme", &postv1.CreatePostRequest{AuthorId: 1, Content: "hello", MediaUrl: "ftp://example.com/a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTestServer(&fakeRepository{}, &fakeCache{}, &fakePublisher{}, nil)
			_, err := server.CreatePost(context.Background(), tt.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument (error %v)", status.Code(err), err)
			}
		})
	}
}

func TestCreatePostCommitsThenRunsSideEffects(t *testing.T) {
	createdAt := time.Date(2026, 8, 12, 1, 2, 3, 0, time.UTC)
	var order []string
	repo := &fakeRepository{createFn: func(_ context.Context, authorID int64, content, mediaURL string) (post.Post, error) {
		order = append(order, "database")
		if authorID != 42 || content != "trimmed" || mediaURL != "https://example.com/image.jpg" {
			t.Fatalf("unexpected create values: %d %q %q", authorID, content, mediaURL)
		}
		return post.Post{ID: 7, AuthorID: authorID, Content: content, MediaURL: mediaURL, CreatedAt: createdAt}, nil
	}}
	cache := &fakeCache{setFn: func(_ context.Context, p post.Post) error {
		order = append(order, "cache")
		if p.ID != 7 {
			t.Fatalf("cached post ID = %d, want 7", p.ID)
		}
		return nil
	}}
	publisher := &fakePublisher{createdFn: func(_ context.Context, p post.Post) error {
		order = append(order, "kafka")
		return nil
	}}

	response, err := newTestServer(repo, cache, publisher, nil).CreatePost(context.Background(), &postv1.CreatePostRequest{
		AuthorId: 42, Content: " trimmed ", MediaUrl: " https://example.com/image.jpg ",
	})
	if err != nil {
		t.Fatalf("CreatePost() error = %v", err)
	}
	if response.GetPostId() != 7 || response.GetCreatedAtUnixMs() != createdAt.UnixMilli() {
		t.Fatalf("response = %+v", response)
	}
	if got := strings.Join(order, ","); got != "database,cache,kafka" {
		t.Fatalf("operation order = %q, want database,cache,kafka", got)
	}
}

func TestCreatePostPostCommitFailuresAreLoggedAndReturnSuccess(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	p := post.Post{ID: 9, AuthorID: 2, Content: "saved", CreatedAt: time.Now()}
	repo := &fakeRepository{createFn: func(context.Context, int64, string, string) (post.Post, error) { return p, nil }}
	cache := &fakeCache{setFn: func(context.Context, post.Post) error { return errors.New("redis unavailable") }}
	publisher := &fakePublisher{createdFn: func(context.Context, post.Post) error { return errors.New("kafka unavailable") }}

	response, err := newTestServer(repo, cache, publisher, logger).CreatePost(context.Background(), &postv1.CreatePostRequest{AuthorId: 2, Content: "saved"})
	if err != nil || response.GetPostId() != 9 {
		t.Fatalf("CreatePost() = (%v, %v), want successful committed post", response, err)
	}
	for _, message := range []string{"post committed but cache write failed", "post committed but PostCreated publish failed"} {
		if !strings.Contains(logs.String(), message) {
			t.Errorf("logs do not contain %q: %s", message, logs.String())
		}
	}
}

func TestCreatePostRepositoryFailuresSkipSideEffects(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{"database failure", errors.New("database down"), codes.Internal},
		{"missing author foreign key", &pgconn.PgError{Code: "23503"}, codes.NotFound},
		{"canceled", context.Canceled, codes.Canceled},
		{"deadline", context.DeadlineExceeded, codes.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := &fakeCache{}
			publisher := &fakePublisher{}
			repo := &fakeRepository{createFn: func(context.Context, int64, string, string) (post.Post, error) {
				return post.Post{}, tt.err
			}}
			_, err := newTestServer(repo, cache, publisher, nil).CreatePost(context.Background(), &postv1.CreatePostRequest{AuthorId: 1, Content: "x"})
			if status.Code(err) != tt.code {
				t.Fatalf("code = %v, want %v", status.Code(err), tt.code)
			}
			if cache.setCalls != 0 || publisher.createdCalls != 0 {
				t.Fatal("side effects ran before a successful database commit")
			}
		})
	}
}

func TestGetPostsPreservesOrderAndUsesCacheAside(t *testing.T) {
	now := time.Now().UTC()
	cache := &fakeCache{getManyFn: func(_ context.Context, ids []int64) (map[int64]post.Post, error) {
		assertIDs(t, ids, []int64{3, 1, 2})
		return map[int64]post.Post{1: {ID: 1, Content: "cached", CreatedAt: now}}, nil
	}}
	repo := &fakeRepository{getFn: func(_ context.Context, ids []int64) (map[int64]post.Post, error) {
		assertIDs(t, ids, []int64{3, 2})
		// ID 2 is absent, representing a missing or soft-deleted post.
		return map[int64]post.Post{3: {ID: 3, Content: "database", CreatedAt: now}}, nil
	}}

	response, err := newTestServer(repo, cache, &fakePublisher{}, nil).GetPosts(context.Background(), &postv1.GetPostsRequest{PostIds: []int64{3, 1, 2, 3}})
	if err != nil {
		t.Fatalf("GetPosts() error = %v", err)
	}
	got := make([]int64, len(response.Posts))
	for i, p := range response.Posts {
		got[i] = p.GetId()
	}
	assertIDs(t, got, []int64{3, 1, 3})
	if repo.getCalls != 1 {
		t.Fatalf("repository calls = %d, want one batch query", repo.getCalls)
	}
	assertIDs(t, postIDs(cache.setManyValue), []int64{3})
}

func TestGetPostsCacheFailureFallsBackToSingleRepositoryCall(t *testing.T) {
	cache := &fakeCache{getManyFn: func(context.Context, []int64) (map[int64]post.Post, error) {
		return nil, errors.New("redis down")
	}}
	repo := &fakeRepository{getFn: func(_ context.Context, ids []int64) (map[int64]post.Post, error) {
		assertIDs(t, ids, []int64{1, 2})
		return map[int64]post.Post{1: {ID: 1}, 2: {ID: 2}}, nil
	}}
	response, err := newTestServer(repo, cache, &fakePublisher{}, nil).GetPosts(context.Background(), &postv1.GetPostsRequest{PostIds: []int64{1, 2}})
	if err != nil || len(response.GetPosts()) != 2 || repo.getCalls != 1 {
		t.Fatalf("GetPosts() = (%v, %v), repository calls %d", response, err, repo.getCalls)
	}
}

func TestGetPostsErrors(t *testing.T) {
	tests := []struct {
		name string
		req  *postv1.GetPostsRequest
		repo *fakeRepository
		code codes.Code
	}{
		{"nil request", nil, &fakeRepository{}, codes.InvalidArgument},
		{"empty IDs", &postv1.GetPostsRequest{}, &fakeRepository{}, codes.InvalidArgument},
		{"nonpositive ID", &postv1.GetPostsRequest{PostIds: []int64{0}}, &fakeRepository{}, codes.InvalidArgument},
		{"repository failure", &postv1.GetPostsRequest{PostIds: []int64{1}}, &fakeRepository{getFn: func(context.Context, []int64) (map[int64]post.Post, error) {
			return nil, errors.New("database down")
		}}, codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTestServer(tt.repo, &fakeCache{}, &fakePublisher{}, nil).GetPosts(context.Background(), tt.req)
			if status.Code(err) != tt.code {
				t.Fatalf("code = %v, want %v", status.Code(err), tt.code)
			}
		})
	}
}

func TestDeletePost(t *testing.T) {
	deletedAt := time.Now().UTC()
	tests := []struct {
		name      string
		req       *postv1.DeletePostRequest
		deleteErr error
		wantCode  codes.Code
		wantSide  bool
	}{
		{"nil request", nil, nil, codes.InvalidArgument, false},
		{"invalid ID", &postv1.DeletePostRequest{PostId: 0, RequestingUserId: 1}, nil, codes.InvalidArgument, false},
		{"not found", &postv1.DeletePostRequest{PostId: 1, RequestingUserId: 2}, repository.ErrNotFound, codes.NotFound, false},
		{"not author", &postv1.DeletePostRequest{PostId: 1, RequestingUserId: 2}, repository.ErrForbidden, codes.PermissionDenied, false},
		{"database failure", &postv1.DeletePostRequest{PostId: 1, RequestingUserId: 2}, errors.New("down"), codes.Internal, false},
		{"success", &postv1.DeletePostRequest{PostId: 1, RequestingUserId: 2}, nil, codes.OK, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{deleteFn: func(context.Context, int64, int64) (int64, time.Time, error) {
				return 2, deletedAt, tt.deleteErr
			}}
			cache := &fakeCache{}
			publisher := &fakePublisher{}
			response, err := newTestServer(repo, cache, publisher, nil).DeletePost(context.Background(), tt.req)
			if status.Code(err) != tt.wantCode {
				t.Fatalf("code = %v, want %v (error %v)", status.Code(err), tt.wantCode, err)
			}
			if tt.wantSide {
				if !response.GetOk() || cache.deleteCalls != 1 || publisher.deletedCalls != 1 {
					t.Fatalf("response/side effects = %+v/%d/%d", response, cache.deleteCalls, publisher.deletedCalls)
				}
			} else if cache.deleteCalls != 0 || publisher.deletedCalls != 0 {
				t.Fatal("side effects ran without successful delete commit")
			}
		})
	}
}

func TestDeletePostPostCommitFailuresReturnSuccess(t *testing.T) {
	repo := &fakeRepository{deleteFn: func(context.Context, int64, int64) (int64, time.Time, error) {
		return 5, time.Now(), nil
	}}
	cache := &fakeCache{deleteFn: func(context.Context, int64) error { return errors.New("redis down") }}
	publisher := &fakePublisher{deletedFn: func(context.Context, int64, int64, time.Time) error { return errors.New("kafka down") }}
	response, err := newTestServer(repo, cache, publisher, nil).DeletePost(context.Background(), &postv1.DeletePostRequest{PostId: 8, RequestingUserId: 5})
	if err != nil || !response.GetOk() {
		t.Fatalf("DeletePost() = (%v, %v), want success", response, err)
	}
}

type fakeRepository struct {
	createFn    func(context.Context, int64, string, string) (post.Post, error)
	getFn       func(context.Context, []int64) (map[int64]post.Post, error)
	deleteFn    func(context.Context, int64, int64) (int64, time.Time, error)
	getCalls    int
	deleteCalls int
}

func (f *fakeRepository) Create(ctx context.Context, authorID int64, content, mediaURL string) (post.Post, error) {
	if f.createFn != nil {
		return f.createFn(ctx, authorID, content, mediaURL)
	}
	return post.Post{}, nil
}
func (f *fakeRepository) GetByIDs(ctx context.Context, ids []int64) (map[int64]post.Post, error) {
	f.getCalls++
	if f.getFn != nil {
		return f.getFn(ctx, ids)
	}
	return map[int64]post.Post{}, nil
}
func (f *fakeRepository) Delete(ctx context.Context, postID, userID int64) (int64, time.Time, error) {
	f.deleteCalls++
	if f.deleteFn != nil {
		return f.deleteFn(ctx, postID, userID)
	}
	return userID, time.Now(), nil
}

type fakeCache struct {
	setFn        func(context.Context, post.Post) error
	getManyFn    func(context.Context, []int64) (map[int64]post.Post, error)
	deleteFn     func(context.Context, int64) error
	setCalls     int
	deleteCalls  int
	setManyValue []post.Post
}

func (f *fakeCache) Set(ctx context.Context, p post.Post) error {
	f.setCalls++
	if f.setFn != nil {
		return f.setFn(ctx, p)
	}
	return nil
}
func (f *fakeCache) SetMany(_ context.Context, posts []post.Post) error {
	f.setManyValue = posts
	return nil
}
func (f *fakeCache) GetMany(ctx context.Context, ids []int64) (map[int64]post.Post, error) {
	if f.getManyFn != nil {
		return f.getManyFn(ctx, ids)
	}
	return map[int64]post.Post{}, nil
}
func (f *fakeCache) DeleteAndTombstone(ctx context.Context, id int64) error {
	f.deleteCalls++
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}

type fakePublisher struct {
	createdFn    func(context.Context, post.Post) error
	deletedFn    func(context.Context, int64, int64, time.Time) error
	createdCalls int
	deletedCalls int
}

func (f *fakePublisher) PublishCreated(ctx context.Context, p post.Post) error {
	f.createdCalls++
	if f.createdFn != nil {
		return f.createdFn(ctx, p)
	}
	return nil
}
func (f *fakePublisher) PublishDeleted(ctx context.Context, postID, authorID int64, at time.Time) error {
	f.deletedCalls++
	if f.deletedFn != nil {
		return f.deletedFn(ctx, postID, authorID, at)
	}
	return nil
}

func newTestServer(repo Repository, cache Cache, publisher EventPublisher, logger *slog.Logger) *Server {
	return New(repo, cache, publisher, logger)
}

func assertIDs(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("IDs = %v, want %v", got, want)
		}
	}
}

func postIDs(posts []post.Post) []int64 {
	ids := make([]int64, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}
	return ids
}
