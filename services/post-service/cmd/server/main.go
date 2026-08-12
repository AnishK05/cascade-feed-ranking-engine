// Command server runs Post Service: a gRPC server that owns post content, writes it to
// Postgres, write-through caches it in Redis, and publishes PostCreated/PostDeleted events to
// Kafka for the Fanout Worker to consume. See IMPLEMENTATION_PLAN.md §5.1.
package main

import (
	"log"
	"net"

	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/config"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/postserver"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()

	lis, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		log.Fatalf("post-service: failed to listen on %s: %v", cfg.Addr(), err)
	}

	grpcServer := grpc.NewServer()
	postv1.RegisterPostServiceServer(grpcServer, postserver.New())

	log.Printf("post-service: listening on %s", cfg.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("post-service: server stopped: %v", err)
	}
}
