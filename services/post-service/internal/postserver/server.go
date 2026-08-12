// Package postserver implements the gRPC PostService defined in proto/post.proto.
//
// Phase 0 only wires up the server skeleton so the service builds, links against the
// generated proto stubs, and has something for CI to compile/test. The real implementation
// (Postgres-backed CreatePost/GetPosts/DeletePost, Redis write-through cache, Kafka publish)
// is added in Phase 3. See IMPLEMENTATION_PLAN.md §5.1.
package postserver

import (
	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
)

// Server implements postv1.PostServiceServer. Embedding the Unimplemented* stub means the
// server satisfies the interface today and will keep compiling as new RPCs are added to the
// proto contract in the future, even before they're implemented here.
type Server struct {
	postv1.UnimplementedPostServiceServer
}

// New constructs a Server. It takes no arguments yet; Phase 3 will extend this to accept a
// Postgres pool, Redis client, and Kafka producer.
func New() *Server {
	return &Server{}
}
