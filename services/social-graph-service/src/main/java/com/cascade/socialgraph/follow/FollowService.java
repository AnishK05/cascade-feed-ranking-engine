package com.cascade.socialgraph.follow;

import com.cascade.socialgraph.api.BadRequestException;
import com.cascade.socialgraph.api.ConflictException;
import com.cascade.socialgraph.api.NotFoundException;
import com.cascade.socialgraph.user.User;
import com.cascade.socialgraph.user.UserRepository;
import com.cascade.socialgraph.user.UserResponse;
import java.time.Instant;
import java.util.List;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

@Service
public class FollowService {

  private static final Logger log = LoggerFactory.getLogger(FollowService.class);

  private final UserRepository userRepository;
  private final FollowRepository followRepository;
  private final FollowEventPublisher eventPublisher;
  private final CursorCodec cursorCodec;
  private final long celebrityThreshold;

  public FollowService(
      UserRepository userRepository,
      FollowRepository followRepository,
      FollowEventPublisher eventPublisher,
      CursorCodec cursorCodec,
      @Value("${cascade.celebrity-threshold:10000}") long celebrityThreshold) {
    this.userRepository = userRepository;
    this.followRepository = followRepository;
    this.eventPublisher = eventPublisher;
    this.cursorCodec = cursorCodec;
    this.celebrityThreshold = celebrityThreshold;
  }

  @Transactional
  public FollowResponse follow(FollowRequest request) {
    validateDifferentUsers(request.followerId(), request.followeeId());
    User follower = requireUser(request.followerId());
    User followee = lockUser(request.followeeId());
    FollowId id = new FollowId(request.followerId(), request.followeeId());
    if (followRepository.existsById(id)) {
      throw new ConflictException("Follow relationship already exists");
    }

    Follow follow;
    try {
      follow = followRepository.saveAndFlush(new Follow(follower, followee));
    } catch (DataIntegrityViolationException exception) {
      throw new ConflictException("Follow relationship already exists");
    }
    followee.incrementFollowers(celebrityThreshold);
    long timestamp = follow.getCreatedAt().toInstant().toEpochMilli();
    publishAfterCommit(new FollowCreated(request.followerId(), request.followeeId(), timestamp));
    return new FollowResponse(request.followerId(), request.followeeId(), follow.getCreatedAt());
  }

  @Transactional
  public void unfollow(long followerId, long followeeId) {
    validateDifferentUsers(followerId, followeeId);
    requireUser(followerId);
    User followee = lockUser(followeeId);
    FollowId id = new FollowId(followerId, followeeId);
    Follow follow =
        followRepository
            .findById(id)
            .orElseThrow(() -> new NotFoundException("Follow relationship not found"));
    followRepository.delete(follow);
    followee.decrementFollowers(celebrityThreshold);
    publishAfterCommit(
        new FollowDeleted(followerId, followeeId, Instant.now().toEpochMilli()));
  }

  @Transactional(readOnly = true)
  public CursorPage<UserResponse> followers(long userId, String cursor, int limit) {
    requireUser(userId);
    return page(
        followRepository.findFollowersAfter(
            userId, cursorCodec.decode(cursor), PageRequest.of(0, limit + 1)),
        limit);
  }

  @Transactional(readOnly = true)
  public CursorPage<UserResponse> following(long userId, String cursor, int limit) {
    requireUser(userId);
    return page(
        followRepository.findFollowingAfter(
            userId, cursorCodec.decode(cursor), PageRequest.of(0, limit + 1)),
        limit);
  }

  private CursorPage<UserResponse> page(List<User> users, int limit) {
    boolean hasMore = users.size() > limit;
    List<User> selected = hasMore ? users.subList(0, limit) : users;
    List<UserResponse> items = selected.stream().map(UserResponse::from).toList();
    String nextCursor =
        hasMore ? cursorCodec.encode(selected.get(selected.size() - 1).getId()) : null;
    return new CursorPage<>(items, nextCursor);
  }

  private void validateDifferentUsers(long followerId, long followeeId) {
    if (followerId == followeeId) {
      throw new BadRequestException("Users cannot follow themselves");
    }
  }

  private User requireUser(long id) {
    return userRepository
        .findById(id)
        .orElseThrow(() -> new NotFoundException("User not found: " + id));
  }

  private User lockUser(long id) {
    return userRepository
        .findByIdForUpdate(id)
        .orElseThrow(() -> new NotFoundException("User not found: " + id));
  }

  private void publishAfterCommit(FollowEvent event) {
    Runnable publish =
        () -> {
          try {
            eventPublisher.publish(event);
          } catch (RuntimeException exception) {
            log.error("Failed to publish {} for followee {}", event.eventType(), event.followeeId(), exception);
          }
        };
    if (TransactionSynchronizationManager.isSynchronizationActive()) {
      TransactionSynchronizationManager.registerSynchronization(
          new TransactionSynchronization() {
            @Override
            public void afterCommit() {
              publish.run();
            }
          });
    } else {
      publish.run();
    }
  }
}
