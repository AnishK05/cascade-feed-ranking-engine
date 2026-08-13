package com.cascade.gateway.social;

import java.time.OffsetDateTime;

public record UserView(
    long id,
    String username,
    String displayName,
    boolean isCelebrity,
    long followerCount,
    OffsetDateTime createdAt) {}
