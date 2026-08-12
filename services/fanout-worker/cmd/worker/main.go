// Command worker runs the Fanout Worker: a Kafka consumer group that fans posts out to
// follower timeline caches in Redis (fanout-on-write), with a hybrid fallback to
// fanout-on-read for celebrity accounts. See IMPLEMENTATION_PLAN.md §5.3.
//
// Phase 0 only wires up configuration loading and the pure decision logic in
// internal/fanout; the actual Kafka consumer loop is added in Phase 5 once the Kafka
// backbone exists (Phase 4).
package main

import (
	"log"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/config"
)

func main() {
	cfg := config.Load()

	log.Printf(
		"fanout-worker: starting (kafka_brokers=%s celebrity_follower_threshold=%d max_timeline_len=%d)",
		cfg.KafkaBrokers, cfg.CelebrityFollowerThreshold, cfg.MaxTimelineLen,
	)
	log.Println("fanout-worker: Kafka consumer loop not yet implemented (see Phase 5)")
}
