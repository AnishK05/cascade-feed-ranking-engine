package com.cascade.socialgraph.follow;

import com.cascade.socialgraph.user.User;
import jakarta.persistence.Column;
import jakarta.persistence.EmbeddedId;
import jakarta.persistence.Entity;
import jakarta.persistence.FetchType;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.ManyToOne;
import jakarta.persistence.MapsId;
import jakarta.persistence.Table;
import java.time.OffsetDateTime;
import org.hibernate.annotations.CreationTimestamp;

@Entity
@Table(name = "follows", schema = "public")
public class Follow {

  @EmbeddedId private FollowId id;

  @MapsId("followerId")
  @ManyToOne(fetch = FetchType.LAZY, optional = false)
  @JoinColumn(name = "follower_id", nullable = false)
  private User follower;

  @MapsId("followeeId")
  @ManyToOne(fetch = FetchType.LAZY, optional = false)
  @JoinColumn(name = "followee_id", nullable = false)
  private User followee;

  @CreationTimestamp
  @Column(name = "created_at", nullable = false, updatable = false)
  private OffsetDateTime createdAt;

  protected Follow() {}

  public Follow(User follower, User followee) {
    this.id = new FollowId(follower.getId(), followee.getId());
    this.follower = follower;
    this.followee = followee;
  }

  public FollowId getId() {
    return id;
  }

  public OffsetDateTime getCreatedAt() {
    return createdAt;
  }
}
