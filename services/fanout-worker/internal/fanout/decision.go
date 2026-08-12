// Package fanout contains the small, pure decision logic behind the hybrid fanout strategy:
// fanout-on-write for ordinary accounts, fanout-on-read for celebrity accounts. Keeping this
// decision in its own dependency-free function makes the core algorithmic idea of the whole
// project unit-testable without a running Kafka/Redis/Postgres. See IMPLEMENTATION_PLAN.md §5.3.
package fanout

// ShouldFanoutOnWrite reports whether a post from an author with the given follower count
// should be pushed into every follower's timeline cache at write time (fanout-on-write), as
// opposed to being left for readers to merge in at read time (fanout-on-read).
//
// Authors at or above threshold are treated as celebrities: fanning out to millions of
// followers on every post would make writes arbitrarily expensive, so their posts are instead
// fanned out on read (see the Fanout Worker's handling of the celebrity_posts:global ZSET).
func ShouldFanoutOnWrite(followerCount, threshold int64) bool {
	return followerCount < threshold
}
