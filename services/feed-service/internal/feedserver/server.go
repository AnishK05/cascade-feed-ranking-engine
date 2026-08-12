// Package feedserver implements the gRPC FeedService defined in proto/feed.proto.
//
// Phase 0 only wires up the server skeleton so the service builds, links against the
// generated proto stubs, and has something for CI to compile/test. The real implementation
// (Redis timeline read, celebrity fanout-on-read merge, cache-miss hydration, ranking,
// pagination) is added in Phase 6. See IMPLEMENTATION_PLAN.md §5.4.
package feedserver

import (
	feedv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/feed/v1"
)

// Server implements feedv1.FeedServiceServer.
type Server struct {
	feedv1.UnimplementedFeedServiceServer
}

// New constructs a Server. It takes no arguments yet; Phase 6 will extend this to accept a
// Redis client, a Post Service gRPC client, and a ranking configuration.
func New() *Server {
	return &Server{}
}
