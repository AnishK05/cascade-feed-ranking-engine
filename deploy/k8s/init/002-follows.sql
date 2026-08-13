-- Owned by Social Graph Service (IMPLEMENTATION_PLAN.md §4, §5.2).
CREATE TABLE follows (
    follower_id     BIGINT NOT NULL REFERENCES users(id),
    followee_id     BIGINT NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id)
);

-- Critical index for fanout: "give me all followers of X" (Fanout Worker, §5.3).
CREATE INDEX idx_follows_followee ON follows(followee_id);

-- Critical index for the read-time celebrity merge: "who does this user follow" (Feed
-- Service, §5.4).
CREATE INDEX idx_follows_follower ON follows(follower_id);
