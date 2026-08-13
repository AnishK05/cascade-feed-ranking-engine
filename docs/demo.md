# Demo walkthrough (5 minutes)

Stack must already be up (`make up` / `cascade.cmd up`) and smoke must have passed
(`ok: follower … saw post …`). Browser: **http://localhost:3000**.

There is **no login**. The top-bar switcher sets `X-User-Id`. That header is spoofable;
this is a local demo, not a product.

## 1. Feed (`/feed`)

The home page redirects here.

1. Open the user switcher. After smoke you should see at least the two smoke users
   (`smoke_author_…`, `smoke_follower_…`). After `make seed` / `cascade.cmd seed` you
   will see hundreds of `user_N` rows.
2. Pick the **author**. Type a short post and submit. It should prepend on this client
   immediately (optimistic). Followers do not have it until Kafka fanout lands.
3. Switch to the **follower**. Refresh or wait a second. The new post should appear,
   ranked (not strictly newest-first if engagement/affinity differ).

If the follower never sees it: fanout or Kafka is unhealthy
(`docker compose -f deploy/docker-compose.yml logs fanout-worker`).

## 2. Graph (`/graph`)

1. As the follower, follow another seeded user (or unfollow and follow again).
2. New follows of **normal** (non-celebrity) accounts backfill recent posts into that
   follower’s Redis timeline. Celebrity follows are merged at read time instead.

## 3. Admin (`/admin`)

Hit `/feed` a few times, then open `/admin`. You should see moving:

- Feed request rate / latency
- Cache hit ratio (after `warm-cache` this should not sit at zero)
- Fanout / Kafka consumer lag

Grafana on http://localhost:3001 is the same signals on a dashboard (`admin`/`admin`,
or anonymous viewer).

## 4. What you are looking at

```text
Browser  →  Gateway :8080  →  Post gRPC / Feed gRPC / Social Graph REST
Post     →  Postgres + Redis + Kafka(post-events)
Fanout   →  Redis timelines (or celebrity_posts:global)
Feed     →  Redis, then Postgres / Post Service on miss, then heuristic rank
```

Hybrid fanout: authors with `follower_count < CELEBRITY_THRESHOLD` (default 10,000;
**80** after `ci` seed) are written to every follower ZSET. Celebrities are one global
ZSET, merged when you follow them.

## 5. Tear down when you are done

`make down` or `cascade.cmd down`. Add `-v` / `down-v` to wipe data volumes.
