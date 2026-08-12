package com.cascade.socialgraph.follow;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.cascade.socialgraph.api.BadRequestException;
import com.cascade.socialgraph.api.ConflictException;
import com.cascade.socialgraph.api.NotFoundException;
import com.cascade.socialgraph.user.User;
import com.cascade.socialgraph.user.UserRepository;
import java.time.OffsetDateTime;
import java.util.Optional;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.support.TransactionSynchronizationManager;

@ExtendWith(MockitoExtension.class)
class FollowServiceTests {

  @Mock private UserRepository userRepository;
  @Mock private FollowRepository followRepository;
  @Mock private FollowEventPublisher eventPublisher;

  private FollowService service;

  @BeforeEach
  void setUp() {
    service =
        new FollowService(
            userRepository, followRepository, eventPublisher, new CursorCodec(), 1);
  }

  @Test
  void rejectsSelfFollow() {
    assertThatThrownBy(() -> service.follow(new FollowRequest(7, 7)))
        .isInstanceOf(BadRequestException.class);
    verify(followRepository, never()).save(any());
  }

  @Test
  void rejectsMissingFollower() {
    when(userRepository.findById(1L)).thenReturn(Optional.empty());

    assertThatThrownBy(() -> service.follow(new FollowRequest(1, 2)))
        .isInstanceOf(NotFoundException.class)
        .hasMessage("User not found: 1");
  }

  @Test
  void rejectsMissingFollowee() {
    when(userRepository.findById(1L)).thenReturn(Optional.of(user(1, 0)));
    when(userRepository.findByIdForUpdate(2L)).thenReturn(Optional.empty());

    assertThatThrownBy(() -> service.follow(new FollowRequest(1, 2)))
        .isInstanceOf(NotFoundException.class)
        .hasMessage("User not found: 2");
  }

  @Test
  void rejectsDuplicateFollowWithoutChangingCount() {
    User followee = user(2, 0);
    when(userRepository.findById(1L)).thenReturn(Optional.of(user(1, 0)));
    when(userRepository.findByIdForUpdate(2L)).thenReturn(Optional.of(followee));
    when(followRepository.existsById(new FollowId(1, 2))).thenReturn(true);

    assertThatThrownBy(() -> service.follow(new FollowRequest(1, 2)))
        .isInstanceOf(ConflictException.class);
    assertThat(followee.getFollowerCount()).isZero();
    verify(eventPublisher, never()).publish(any());
  }

  @Test
  void followCrossesCelebrityThresholdAndPublishesCreatedEvent() {
    User follower = user(1, 0);
    User followee = user(2, 0);
    when(userRepository.findById(1L)).thenReturn(Optional.of(follower));
    when(userRepository.findByIdForUpdate(2L)).thenReturn(Optional.of(followee));
    when(followRepository.existsById(new FollowId(1, 2))).thenReturn(false);
    when(followRepository.saveAndFlush(any(Follow.class)))
        .thenAnswer(
            invocation -> {
              Follow follow = invocation.getArgument(0);
              ReflectionTestUtils.setField(follow, "createdAt", OffsetDateTime.now());
              return follow;
            });

    FollowResponse response = service.follow(new FollowRequest(1, 2));

    assertThat(response.followerId()).isEqualTo(1);
    assertThat(followee.getFollowerCount()).isEqualTo(1);
    assertThat(followee.isCelebrity()).isTrue();
    ArgumentCaptor<FollowEvent> event = ArgumentCaptor.forClass(FollowEvent.class);
    verify(eventPublisher).publish(event.capture());
    assertThat(event.getValue()).isInstanceOf(FollowCreated.class);
    assertThat(event.getValue().followeeId()).isEqualTo(2);
  }

  @Test
  void defersEventUntilTransactionCommit() {
    User follower = user(1, 0);
    User followee = user(2, 0);
    when(userRepository.findById(1L)).thenReturn(Optional.of(follower));
    when(userRepository.findByIdForUpdate(2L)).thenReturn(Optional.of(followee));
    when(followRepository.saveAndFlush(any(Follow.class)))
        .thenAnswer(
            invocation -> {
              Follow follow = invocation.getArgument(0);
              ReflectionTestUtils.setField(follow, "createdAt", OffsetDateTime.now());
              return follow;
            });

    TransactionSynchronizationManager.initSynchronization();
    try {
      service.follow(new FollowRequest(1, 2));
      verify(eventPublisher, never()).publish(any());

      TransactionSynchronizationManager.getSynchronizations()
          .forEach(synchronization -> synchronization.afterCommit());

      verify(eventPublisher).publish(any(FollowCreated.class));
    } finally {
      TransactionSynchronizationManager.clearSynchronization();
    }
  }

  @Test
  void unfollowDecrementsCountClearsCelebrityAndPublishesDeletedEvent() {
    User follower = user(1, 0);
    User followee = user(2, 1);
    Follow follow = new Follow(follower, followee);
    when(userRepository.findById(1L)).thenReturn(Optional.of(follower));
    when(userRepository.findByIdForUpdate(2L)).thenReturn(Optional.of(followee));
    when(followRepository.findById(new FollowId(1, 2))).thenReturn(Optional.of(follow));

    service.unfollow(1, 2);

    assertThat(followee.getFollowerCount()).isZero();
    assertThat(followee.isCelebrity()).isFalse();
    verify(followRepository).delete(follow);
    ArgumentCaptor<FollowEvent> event = ArgumentCaptor.forClass(FollowEvent.class);
    verify(eventPublisher).publish(event.capture());
    assertThat(event.getValue()).isInstanceOf(FollowDeleted.class);
  }

  @Test
  void rejectsUnfollowWhenRelationshipDoesNotExist() {
    User follower = user(1, 0);
    User followee = user(2, 1);
    when(userRepository.findById(1L)).thenReturn(Optional.of(follower));
    when(userRepository.findByIdForUpdate(2L)).thenReturn(Optional.of(followee));
    when(followRepository.findById(new FollowId(1, 2))).thenReturn(Optional.empty());

    assertThatThrownBy(() -> service.unfollow(1, 2)).isInstanceOf(NotFoundException.class);
    assertThat(followee.getFollowerCount()).isEqualTo(1);
  }

  private User user(long id, long followerCount) {
    User user = new User("user_" + id, "User " + id);
    ReflectionTestUtils.setField(user, "id", id);
    ReflectionTestUtils.setField(user, "followerCount", followerCount);
    ReflectionTestUtils.setField(user, "celebrity", followerCount >= 1);
    ReflectionTestUtils.setField(user, "createdAt", OffsetDateTime.now());
    return user;
  }
}
