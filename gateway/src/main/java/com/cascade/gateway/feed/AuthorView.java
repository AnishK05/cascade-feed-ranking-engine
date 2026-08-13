package com.cascade.gateway.feed;

import com.cascade.gateway.social.UserView;

public record AuthorView(
    long id, String username, String displayName, boolean celebrity) {

  public static AuthorView from(UserView user) {
    return new AuthorView(user.id(), user.username(), user.displayName(), user.isCelebrity());
  }
}
