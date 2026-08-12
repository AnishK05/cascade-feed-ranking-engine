package com.cascade.socialgraph.follow;

import jakarta.validation.constraints.Positive;

public record FollowRequest(@Positive long followerId, @Positive long followeeId) {}
