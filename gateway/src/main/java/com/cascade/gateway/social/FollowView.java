package com.cascade.gateway.social;

import java.time.OffsetDateTime;

public record FollowView(long followerId, long followeeId, OffsetDateTime createdAt) {}
