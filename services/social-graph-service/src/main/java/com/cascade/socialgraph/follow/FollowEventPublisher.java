package com.cascade.socialgraph.follow;

public interface FollowEventPublisher {
  void publish(FollowEvent event);
}
