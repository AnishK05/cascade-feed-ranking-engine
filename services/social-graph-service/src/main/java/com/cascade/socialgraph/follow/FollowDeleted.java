package com.cascade.socialgraph.follow;

public record FollowDeleted(
    String eventType, long followerId, long followeeId, long deletedAtUnixMs)
    implements FollowEvent {

  public FollowDeleted(long followerId, long followeeId, long deletedAtUnixMs) {
    this("FollowDeleted", followerId, followeeId, deletedAtUnixMs);
  }
}
