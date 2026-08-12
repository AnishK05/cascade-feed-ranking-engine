-- Owned by Social Graph Service (IMPLEMENTATION_PLAN.md §4, §5.2).
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    username        TEXT UNIQUE NOT NULL,
    display_name    TEXT NOT NULL,
    -- Denormalized flag, recomputed by Social Graph Service whenever follower_count crosses
    -- CELEBRITY_THRESHOLD (see §5.2, §5.3). Kept denormalized so the Fanout Worker's celebrity
    -- check doesn't require a COUNT(*) over `follows` on every post.
    is_celebrity    BOOLEAN NOT NULL DEFAULT FALSE,
    -- Denormalized, updated in the same transaction as each follow/unfollow.
    follower_count  BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
