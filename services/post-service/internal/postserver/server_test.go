package postserver

import (
	"context"
	"testing"

	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestServerImplementsInterface pins down that Server satisfies the generated
// PostServiceServer interface at compile time.
func TestServerImplementsInterface(t *testing.T) {
	var _ postv1.PostServiceServer = New()
}

// TestCreatePostUnimplemented documents today's expected behavior: until Phase 3 implements
// the real logic, every RPC should fail with codes.Unimplemented rather than panicking or
// silently succeeding.
func TestCreatePostUnimplemented(t *testing.T) {
	s := New()

	_, err := s.CreatePost(context.Background(), &postv1.CreatePostRequest{
		AuthorId: 1,
		Content:  "hello, world",
	})

	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("CreatePost() error = %v, want status code %v", err, codes.Unimplemented)
	}
}
