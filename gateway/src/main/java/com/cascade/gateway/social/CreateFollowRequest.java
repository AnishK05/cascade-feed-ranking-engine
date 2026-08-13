package com.cascade.gateway.social;

import jakarta.validation.constraints.Positive;

public record CreateFollowRequest(@Positive long followeeId) {}
