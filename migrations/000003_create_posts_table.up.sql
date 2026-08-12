-- Owned by Post Service (IMPLEMENTATION_PLAN.md §4, §5.1).
CREATE TABLE posts (
    id              BIGSERIAL PRIMARY KEY,
    author_id       BIGINT NOT NULL REFERENCES users(id),
    content         TEXT NOT NULL,
    media_url       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Soft delete: Feed Service filters tombstoned post IDs out of cached timelines at read
    -- time rather than trying to purge them from every follower's Redis ZSET, which would
    -- re-introduce the fanout cost the celebrity/hybrid strategy exists to avoid (§7.4).
    deleted_at      TIMESTAMPTZ
);

-- Supports "most recent posts by this author" lookups (e.g. new-follow cache-warming
-- backfill, §7.3, and any direct author-timeline queries).
CREATE INDEX idx_posts_author_created ON posts(author_id, created_at DESC);
