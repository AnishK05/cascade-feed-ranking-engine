package feedserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	feedv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/feed/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feed"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeCandidates struct {
	value feed.CandidateSet
	err   error
}

func (f fakeCandidates) Load(context.Context, int64, int) (feed.CandidateSet, error) {
	return f.value, f.err
}

type fakeHydrator struct {
	value map[int64]feed.Post
	err   error
	ids   []int64
}

func (f *fakeHydrator) Hydrate(_ context.Context, ids []int64) (map[int64]feed.Post, int, int, error) {
	f.ids = append([]int64(nil), ids...)
	return f.value, len(f.value), 0, f.err
}

type fakeSignals struct {
	value map[int64]feed.Signal
	err   error
	posts []feed.Post
}

func (f *fakeSignals) Load(_ context.Context, _ int64, posts []feed.Post) (map[int64]feed.Signal, error) {
	f.posts = append([]feed.Post(nil), posts...)
	return f.value, f.err
}

type idRanker struct{}

func (idRanker) Rank(posts []feed.Post, _ map[int64]feed.Signal) []feed.RankedPost {
	result := make([]feed.RankedPost, len(posts))
	for i, post := range posts {
		result[i] = feed.RankedPost{Post: post, Score: float64(post.ID)}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID > result[j].ID })
	return result
}

func testServer(c CandidateStore, h Hydrator, s SignalProvider) *Server {
	return New(c, h, s, idRanker{}, nil, 200, 20, 100, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestServerImplementsInterface(t *testing.T) {
	var _ feedv1.FeedServiceServer = testServer(fakeCandidates{}, &fakeHydrator{}, &fakeSignals{})
}

func TestGetFeedNormalOnly(t *testing.T) {
	now := time.Now().UTC()
	hydrator := &fakeHydrator{value: map[int64]feed.Post{
		1: {ID: 1, AuthorID: 11, Content: "one", CreatedAt: now},
		2: {ID: 2, AuthorID: 12, Content: "two", CreatedAt: now},
	}}
	server := testServer(
		fakeCandidates{value: feed.CandidateSet{NormalIDs: []int64{1, 2}}},
		hydrator,
		&fakeSignals{},
	)
	response, err := server.GetFeed(context.Background(), &feedv1.GetFeedRequest{UserId: 1})
	if err != nil {
		t.Fatalf("GetFeed() error = %v", err)
	}
	if len(response.GetItems()) != 2 || response.GetItems()[0].GetPostId() != 2 {
		t.Fatalf("GetFeed() items = %+v", response.GetItems())
	}
}

func TestGetFeedCelebrityFilterMergeAndDedupe(t *testing.T) {
	now := time.Now().UTC()
	hydrator := &fakeHydrator{value: map[int64]feed.Post{
		1: {ID: 1, AuthorID: 1, CreatedAt: now},
		2: {ID: 2, AuthorID: 2, CreatedAt: now},
		3: {ID: 3, AuthorID: 99, CreatedAt: now},
		4: {ID: 4, AuthorID: 100, CreatedAt: now},
	}}
	signals := &fakeSignals{}
	server := testServer(fakeCandidates{value: feed.CandidateSet{
		NormalIDs: []int64{1, 2}, CelebrityIDs: []int64{2, 3, 4},
		FollowedCelebrities: map[int64]struct{}{99: {}},
	}}, hydrator, signals)
	response, err := server.GetFeed(context.Background(), &feedv1.GetFeedRequest{UserId: 7})
	if err != nil {
		t.Fatalf("GetFeed() error = %v", err)
	}
	assertIDs(t, hydrator.ids, []int64{1, 2, 3, 4})
	assertItemIDs(t, response.GetItems(), []int64{3, 2, 1})
	if len(signals.posts) != 3 {
		t.Fatalf("signal posts = %+v, want three filtered/deduplicated posts", signals.posts)
	}
}

func TestGetFeedMultiPageHasNoDuplicates(t *testing.T) {
	now := time.Now().UTC()
	candidates := feed.CandidateSet{NormalIDs: []int64{1, 2, 3, 4, 5}}
	posts := make(map[int64]feed.Post)
	for id := int64(1); id <= 5; id++ {
		posts[id] = feed.Post{ID: id, AuthorID: id, CreatedAt: now}
	}
	server := testServer(fakeCandidates{value: candidates}, &fakeHydrator{value: posts}, &fakeSignals{})
	token := ""
	seen := make(map[int64]struct{})
	for {
		response, err := server.GetFeed(context.Background(), &feedv1.GetFeedRequest{
			UserId: 1, PageSize: 2, PageToken: token,
		})
		if err != nil {
			t.Fatalf("GetFeed() error = %v", err)
		}
		for _, item := range response.GetItems() {
			if _, duplicate := seen[item.GetPostId()]; duplicate {
				t.Fatalf("post %d appeared on more than one page", item.GetPostId())
			}
			seen[item.GetPostId()] = struct{}{}
		}
		token = response.GetNextPageToken()
		if token == "" {
			break
		}
	}
	if len(seen) != 5 {
		t.Fatalf("paginated posts = %v, want five", seen)
	}
}

func TestGetFeedValidationAndDependencyErrors(t *testing.T) {
	validCandidates := fakeCandidates{value: feed.CandidateSet{}}
	tests := []struct {
		name string
		req  *feedv1.GetFeedRequest
		c    CandidateStore
		h    Hydrator
		s    SignalProvider
		code codes.Code
	}{
		{"nil request", nil, validCandidates, &fakeHydrator{}, &fakeSignals{}, codes.InvalidArgument},
		{"bad user", &feedv1.GetFeedRequest{}, validCandidates, &fakeHydrator{}, &fakeSignals{}, codes.InvalidArgument},
		{"negative page size", &feedv1.GetFeedRequest{UserId: 1, PageSize: -1}, validCandidates, &fakeHydrator{}, &fakeSignals{}, codes.InvalidArgument},
		{"oversized page", &feedv1.GetFeedRequest{UserId: 1, PageSize: 101}, validCandidates, &fakeHydrator{}, &fakeSignals{}, codes.InvalidArgument},
		{"bad token", &feedv1.GetFeedRequest{UserId: 1, PageToken: "bad"}, validCandidates, &fakeHydrator{}, &fakeSignals{}, codes.InvalidArgument},
		{"candidate failure", &feedv1.GetFeedRequest{UserId: 1}, fakeCandidates{err: errors.New("redis")}, &fakeHydrator{}, &fakeSignals{}, codes.Unavailable},
		{"hydrator failure", &feedv1.GetFeedRequest{UserId: 1}, validCandidates, &fakeHydrator{err: errors.New("post")}, &fakeSignals{}, codes.Unavailable},
		{"signal failure", &feedv1.GetFeedRequest{UserId: 1}, validCandidates, &fakeHydrator{}, &fakeSignals{err: errors.New("postgres")}, codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := testServer(tt.c, tt.h, tt.s).GetFeed(context.Background(), tt.req)
			if status.Code(err) != tt.code {
				t.Fatalf("GetFeed() code = %v, want %v (error %v)", status.Code(err), tt.code, err)
			}
		})
	}
}

func assertIDs(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("IDs = %v, want %v", got, want)
		}
	}
}

func assertItemIDs(t *testing.T, got []*feedv1.FeedItem, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("item count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].GetPostId() != want[i] {
			t.Fatalf("item %d ID = %d, want %d", i, got[i].GetPostId(), want[i])
		}
	}
}
