-- Owned by Post Service (IMPLEMENTATION_PLAN.md §4). Feeds the ranking signal described in
-- §8.1 (engagement counts, author affinity) and the optional offline model in §8.2.
CREATE TABLE engagements (
    id              BIGSERIAL PRIMARY KEY,
    post_id         BIGINT NOT NULL REFERENCES posts(id),
    user_id         BIGINT NOT NULL REFERENCES users(id),
    type            TEXT NOT NULL CHECK (type IN ('like', 'comment', 'view', 'share')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Supports "engagement counts for this post" lookups used by the ranking formula (§8.1).
CREATE INDEX idx_engagements_post ON engagements(post_id);
