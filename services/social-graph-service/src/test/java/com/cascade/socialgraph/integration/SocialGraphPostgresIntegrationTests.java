package com.cascade.socialgraph.integration;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.atLeast;
import static org.mockito.Mockito.verify;

import com.cascade.socialgraph.follow.CursorPage;
import com.cascade.socialgraph.follow.FollowEventPublisher;
import com.cascade.socialgraph.follow.FollowRequest;
import com.cascade.socialgraph.follow.FollowService;
import com.cascade.socialgraph.user.CreateUserRequest;
import com.cascade.socialgraph.user.UserResponse;
import com.cascade.socialgraph.user.UserService;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.postgresql.PostgreSQLContainer;

@Testcontainers(disabledWithoutDocker = true)
@SpringBootTest(properties = "cascade.celebrity-threshold=2")
class SocialGraphPostgresIntegrationTests {

  @Container
  static final PostgreSQLContainer postgres =
      new PostgreSQLContainer("postgres:17-alpine").withInitScript("schema.sql");

  @DynamicPropertySource
  static void databaseProperties(DynamicPropertyRegistry registry) {
    registry.add("DATABASE_URL", postgres::getJdbcUrl);
    registry.add("DB_USER", postgres::getUsername);
    registry.add("DB_PASSWORD", postgres::getPassword);
  }

  @Autowired private UserService userService;
  @Autowired private FollowService followService;
  @MockitoBean private FollowEventPublisher eventPublisher;

  @Test
  void persistsGraphMaintainsCountsPaginatesAndPublishesAfterCommit() {
    UserResponse target = userService.create(new CreateUserRequest("target", "Target"));
    UserResponse first = userService.create(new CreateUserRequest("first", "First"));
    UserResponse second = userService.create(new CreateUserRequest("second", "Second"));

    followService.follow(new FollowRequest(first.id(), target.id()));
    assertThat(userService.get(target.id()).followerCount()).isEqualTo(1);
    assertThat(userService.get(target.id()).isCelebrity()).isFalse();

    followService.follow(new FollowRequest(second.id(), target.id()));
    UserResponse celebrity = userService.get(target.id());
    assertThat(celebrity.followerCount()).isEqualTo(2);
    assertThat(celebrity.isCelebrity()).isTrue();

    CursorPage<UserResponse> page = followService.followers(target.id(), null, 1);
    assertThat(page.items()).hasSize(1);
    assertThat(page.nextCursor()).isNotBlank();
    assertThat(followService.followers(target.id(), page.nextCursor(), 1).items()).hasSize(1);
    assertThat(userService.list(10))
        .extracting(UserResponse::username)
        .contains("target", "first", "second");

    followService.unfollow(second.id(), target.id());
    UserResponse afterUnfollow = userService.get(target.id());
    assertThat(afterUnfollow.followerCount()).isEqualTo(1);
    assertThat(afterUnfollow.isCelebrity()).isFalse();
    verify(eventPublisher, atLeast(3)).publish(any());
  }
}
