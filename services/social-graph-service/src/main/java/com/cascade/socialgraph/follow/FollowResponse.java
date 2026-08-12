package com.cascade.socialgraph.follow;

import java.time.OffsetDateTime;

public record FollowResponse(long followerId, long followeeId, OffsetDateTime createdAt) {}
