package com.cascade.gateway.social;

import com.cascade.gateway.auth.UserIds;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.Positive;
import java.net.URI;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@Validated
@RestController
@RequestMapping("/api")
public class SocialGraphController {

  private final SocialGraphClient socialGraphClient;

  public SocialGraphController(SocialGraphClient socialGraphClient) {
    this.socialGraphClient = socialGraphClient;
  }

  @PostMapping("/users")
  ResponseEntity<UserView> createUser(@Valid @RequestBody CreateUserRequest request) {
    UserView created = socialGraphClient.createUser(request);
    return ResponseEntity.created(URI.create("/api/users/" + created.id())).body(created);
  }

  @GetMapping("/users/{id}")
  UserView getUser(@PathVariable @Positive long id) {
    return socialGraphClient.getUser(id);
  }

  @PostMapping("/follows")
  ResponseEntity<FollowView> follow(
      @RequestHeader(value = UserIds.HEADER, required = false) String userIdHeader,
      @Valid @RequestBody CreateFollowRequest request) {
    long followerId = UserIds.require(userIdHeader);
    FollowView created = socialGraphClient.follow(followerId, request.followeeId());
    return ResponseEntity.created(URI.create("/api/follows/" + request.followeeId())).body(created);
  }

  @DeleteMapping("/follows/{followeeId}")
  ResponseEntity<Void> unfollow(
      @PathVariable @Positive long followeeId,
      @RequestHeader(value = UserIds.HEADER, required = false) String userIdHeader) {
    socialGraphClient.unfollow(UserIds.require(userIdHeader), followeeId);
    return ResponseEntity.noContent().build();
  }

  @GetMapping("/users/{id}/followers")
  CursorPage<UserView> followers(
      @PathVariable @Positive long id,
      @RequestParam(required = false) String cursor,
      @RequestParam(defaultValue = "20") @Min(1) @Max(100) int limit) {
    return socialGraphClient.followers(id, cursor, limit);
  }

  @GetMapping("/users/{id}/following")
  CursorPage<UserView> following(
      @PathVariable @Positive long id,
      @RequestParam(required = false) String cursor,
      @RequestParam(defaultValue = "20") @Min(1) @Max(100) int limit) {
    return socialGraphClient.following(id, cursor, limit);
  }
}
