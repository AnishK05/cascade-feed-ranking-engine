package com.cascade.socialgraph.follow;

public record FollowCreated(
    String eventType, long followerId, long followeeId, long createdAtUnixMs)
    implements FollowEvent {

  public FollowCreated(long followerId, long followeeId, long createdAtUnixMs) {
    this("FollowCreated", followerId, followeeId, createdAtUnixMs);
  }
}
