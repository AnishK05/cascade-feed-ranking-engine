package com.cascade.socialgraph.follow;

import jakarta.persistence.Column;
import jakarta.persistence.Embeddable;
import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class FollowId implements Serializable {

  @Column(name = "follower_id")
  private Long followerId;

  @Column(name = "followee_id")
  private Long followeeId;

  protected FollowId() {}

  public FollowId(long followerId, long followeeId) {
    this.followerId = followerId;
    this.followeeId = followeeId;
  }

  public Long getFollowerId() {
    return followerId;
  }

  public Long getFolloweeId() {
    return followeeId;
  }

  @Override
  public boolean equals(Object other) {
    if (this == other) {
      return true;
    }
    if (!(other instanceof FollowId that)) {
      return false;
    }
    return Objects.equals(followerId, that.followerId)
        && Objects.equals(followeeId, that.followeeId);
  }

  @Override
  public int hashCode() {
    return Objects.hash(followerId, followeeId);
  }
}
