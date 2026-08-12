package feedserver

import (
	"context"
	"testing"

	feedv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/feed/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestServerImplementsInterface(t *testing.T) {
	var _ feedv1.FeedServiceServer = New()
}

func TestGetFeedUnimplemented(t *testing.T) {
	s := New()

	_, err := s.GetFeed(context.Background(), &feedv1.GetFeedRequest{UserId: 1})

	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("GetFeed() error = %v, want status code %v", err, codes.Unimplemented)
	}
}
