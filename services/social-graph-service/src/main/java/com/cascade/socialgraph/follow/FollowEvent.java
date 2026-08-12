package com.cascade.socialgraph.follow;

public interface FollowEvent {
  String eventType();

  long followerId();

  long followeeId();
}
