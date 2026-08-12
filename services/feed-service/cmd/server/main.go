// Command server runs Feed Service: a gRPC server that reads the fanout-on-write timeline
// cache in Redis, merges in fanout-on-read candidates from celebrity accounts, hydrates
// content, ranks, and paginates the result. See IMPLEMENTATION_PLAN.md §5.4.
package main

import (
	"log"
	"net"

	feedv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/feed/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/config"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feedserver"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()

	lis, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		log.Fatalf("feed-service: failed to listen on %s: %v", cfg.Addr(), err)
	}

	grpcServer := grpc.NewServer()
	feedv1.RegisterFeedServiceServer(grpcServer, feedserver.New())

	log.Printf("feed-service: listening on %s", cfg.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("feed-service: server stopped: %v", err)
	}
}
