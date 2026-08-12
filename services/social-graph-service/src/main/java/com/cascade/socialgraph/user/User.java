package com.cascade.socialgraph.user;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import java.time.OffsetDateTime;
import org.hibernate.annotations.CreationTimestamp;

@Entity
@Table(name = "users", schema = "public")
public class User {

  @Id
  @GeneratedValue(strategy = GenerationType.IDENTITY)
  private Long id;

  @Column(nullable = false, unique = true, columnDefinition = "text")
  private String username;

  @Column(name = "display_name", nullable = false, columnDefinition = "text")
  private String displayName;

  @Column(name = "is_celebrity", nullable = false)
  private boolean celebrity;

  @Column(name = "follower_count", nullable = false)
  private long followerCount;

  @CreationTimestamp
  @Column(name = "created_at", nullable = false, updatable = false)
  private OffsetDateTime createdAt;

  protected User() {}

  public User(String username, String displayName) {
    this.username = username;
    this.displayName = displayName;
  }

  public Long getId() {
    return id;
  }

  public String getUsername() {
    return username;
  }

  public String getDisplayName() {
    return displayName;
  }

  public boolean isCelebrity() {
    return celebrity;
  }

  public long getFollowerCount() {
    return followerCount;
  }

  public OffsetDateTime getCreatedAt() {
    return createdAt;
  }

  public void incrementFollowers(long celebrityThreshold) {
    followerCount++;
    celebrity = followerCount >= celebrityThreshold;
  }

  public void decrementFollowers(long celebrityThreshold) {
    followerCount = Math.max(0, followerCount - 1);
    celebrity = followerCount >= celebrityThreshold;
  }
}
