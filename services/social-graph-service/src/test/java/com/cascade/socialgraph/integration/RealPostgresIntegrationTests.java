package com.cascade.socialgraph.integration;

import static org.assertj.core.api.Assertions.assertThat;

import com.cascade.socialgraph.follow.FollowEventPublisher;
import com.cascade.socialgraph.follow.FollowRequest;
import com.cascade.socialgraph.follow.FollowService;
import com.cascade.socialgraph.user.CreateUserRequest;
import com.cascade.socialgraph.user.UserResponse;
import com.cascade.socialgraph.user.UserService;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.bean.override.mockito.MockitoBean;

@EnabledIfEnvironmentVariable(named = "TEST_DATABASE_URL", matches = ".+")
@ActiveProfiles("postgres-test")
@SpringBootTest(properties = "cascade.celebrity-threshold=1")
class RealPostgresIntegrationTests {

  @Autowired private UserService userService;
  @Autowired private FollowService followService;
  @MockitoBean private FollowEventPublisher eventPublisher;

  @Test
  void validatesSchemaAndPersistsFollowLifecycle() {
    String suffix = UUID.randomUUID().toString().replace("-", "");
    UserResponse follower =
        userService.create(new CreateUserRequest("f_" + suffix, "Follower"));
    UserResponse followee =
        userService.create(new CreateUserRequest("t_" + suffix, "Followee"));

    followService.follow(new FollowRequest(follower.id(), followee.id()));
    assertThat(userService.get(followee.id()).followerCount()).isEqualTo(1);
    assertThat(userService.get(followee.id()).isCelebrity()).isTrue();

    followService.unfollow(follower.id(), followee.id());
    assertThat(userService.get(followee.id()).followerCount()).isZero();
    assertThat(userService.get(followee.id()).isCelebrity()).isFalse();
  }

  @Test
  void concurrentFollowsMaintainExactCount() throws Exception {
    String suffix = UUID.randomUUID().toString().replace("-", "");
    UserResponse first =
        userService.create(new CreateUserRequest("a_" + suffix, "First"));
    UserResponse second =
        userService.create(new CreateUserRequest("b_" + suffix, "Second"));
    UserResponse target =
        userService.create(new CreateUserRequest("c_" + suffix, "Target"));
    CountDownLatch start = new CountDownLatch(1);

    try (var executor = Executors.newFixedThreadPool(2)) {
      Future<?> firstFollow =
          executor.submit(
              () -> {
                await(start);
                followService.follow(new FollowRequest(first.id(), target.id()));
              });
      Future<?> secondFollow =
          executor.submit(
              () -> {
                await(start);
                followService.follow(new FollowRequest(second.id(), target.id()));
              });
      start.countDown();
      firstFollow.get();
      secondFollow.get();
    }

    assertThat(userService.get(target.id()).followerCount()).isEqualTo(2);
  }

  private void await(CountDownLatch latch) {
    try {
      latch.await();
    } catch (InterruptedException exception) {
      Thread.currentThread().interrupt();
      throw new IllegalStateException("Interrupted while starting concurrent follows", exception);
    }
  }
}
