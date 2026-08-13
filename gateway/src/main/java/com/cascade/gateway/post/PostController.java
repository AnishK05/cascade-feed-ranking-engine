package com.cascade.gateway.post;

import com.cascade.gateway.api.BadRequestException;
import com.cascade.gateway.auth.UserIds;
import com.cascade.proto.post.v1.CreatePostResponse;
import com.cascade.proto.post.v1.Post;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Positive;
import java.util.ArrayList;
import java.util.List;
import org.springframework.http.HttpStatus;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.ResponseStatus;
import org.springframework.web.bind.annotation.RestController;

@Validated
@RestController
@RequestMapping("/api/posts")
public class PostController {

  private final PostClient postClient;

  public PostController(PostClient postClient) {
    this.postClient = postClient;
  }

  @PostMapping
  @ResponseStatus(HttpStatus.CREATED)
  CreatedPostResponse create(
      @RequestHeader(value = UserIds.HEADER, required = false) String userIdHeader,
      @Valid @RequestBody CreatePostBody body) {
    long userId = UserIds.require(userIdHeader);
    CreatePostResponse created =
        postClient.create(userId, body.content(), body.mediaUrl() == null ? "" : body.mediaUrl());
    return new CreatedPostResponse(created.getPostId(), userId, created.getCreatedAtUnixMs());
  }

  @GetMapping
  List<PostView> getMany(@RequestParam String ids) {
    return postClient.getPosts(parseIds(ids)).stream().map(PostController::toView).toList();
  }

  @DeleteMapping("/{id}")
  @ResponseStatus(HttpStatus.NO_CONTENT)
  void delete(
      @PathVariable @Positive long id,
      @RequestHeader(value = UserIds.HEADER, required = false) String userIdHeader) {
    postClient.delete(id, UserIds.require(userIdHeader));
  }

  private static List<Long> parseIds(String ids) {
    if (ids == null || ids.isBlank()) {
      throw new BadRequestException("ids must contain at least one ID");
    }
    List<Long> postIds = new ArrayList<>();
    for (String part : ids.split(",")) {
      String value = part.trim();
      if (value.isEmpty()) {
        continue;
      }
      try {
        long id = Long.parseLong(value);
        if (id <= 0) {
          throw new BadRequestException("ids must contain only positive IDs");
        }
        postIds.add(id);
      } catch (NumberFormatException exception) {
        throw new BadRequestException("ids must contain only positive IDs");
      }
    }
    if (postIds.isEmpty()) {
      throw new BadRequestException("ids must contain at least one ID");
    }
    return postIds;
  }

  private static PostView toView(Post post) {
    return new PostView(
        post.getId(),
        post.getAuthorId(),
        post.getContent(),
        post.getMediaUrl(),
        post.getCreatedAtUnixMs());
  }
}
