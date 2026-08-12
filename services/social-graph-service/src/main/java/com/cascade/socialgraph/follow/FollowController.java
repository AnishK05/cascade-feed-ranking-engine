package com.cascade.socialgraph.follow;

import com.cascade.socialgraph.user.UserResponse;
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
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@Validated
@RestController
public class FollowController {

  private final FollowService followService;

  public FollowController(FollowService followService) {
    this.followService = followService;
  }

  @PostMapping("/follows")
  ResponseEntity<FollowResponse> follow(@Valid @RequestBody FollowRequest request) {
    FollowResponse response = followService.follow(request);
    return ResponseEntity.created(
            URI.create("/follows/" + response.followerId() + "/" + response.followeeId()))
        .body(response);
  }

  @DeleteMapping("/follows/{followerId}/{followeeId}")
  ResponseEntity<Void> unfollow(
      @PathVariable @Positive long followerId, @PathVariable @Positive long followeeId) {
    followService.unfollow(followerId, followeeId);
    return ResponseEntity.noContent().build();
  }

  @GetMapping("/users/{id}/followers")
  CursorPage<UserResponse> followers(
      @PathVariable @Positive long id,
      @RequestParam(required = false) String cursor,
      @RequestParam(defaultValue = "20") @Min(1) @Max(100) int limit) {
    return followService.followers(id, cursor, limit);
  }

  @GetMapping("/users/{id}/following")
  CursorPage<UserResponse> following(
      @PathVariable @Positive long id,
      @RequestParam(required = false) String cursor,
      @RequestParam(defaultValue = "20") @Min(1) @Max(100) int limit) {
    return followService.following(id, cursor, limit);
  }
}
