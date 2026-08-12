package com.cascade.socialgraph.user;

import java.time.OffsetDateTime;

public record UserResponse(
    long id,
    String username,
    String displayName,
    boolean isCelebrity,
    long followerCount,
    OffsetDateTime createdAt) {

  public static UserResponse from(User user) {
    return new UserResponse(
        user.getId(),
        user.getUsername(),
        user.getDisplayName(),
        user.isCelebrity(),
        user.getFollowerCount(),
        user.getCreatedAt());
  }
}
