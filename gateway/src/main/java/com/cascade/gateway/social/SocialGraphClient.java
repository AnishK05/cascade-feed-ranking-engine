package com.cascade.gateway.social;

import java.util.Collection;
import java.util.List;

public interface SocialGraphClient {

  UserView createUser(CreateUserRequest request);

  UserView getUser(long id);

  List<UserView> getUsers(Collection<Long> ids);

  FollowView follow(long followerId, long followeeId);

  void unfollow(long followerId, long followeeId);

  CursorPage<UserView> followers(long userId, String cursor, int limit);

  CursorPage<UserView> following(long userId, String cursor, int limit);
}
