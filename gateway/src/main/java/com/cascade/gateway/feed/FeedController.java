package com.cascade.gateway.feed;

import com.cascade.gateway.auth.UserIds;
import com.cascade.gateway.social.SocialGraphClient;
import com.cascade.gateway.social.UserView;
import com.cascade.proto.feed.v1.FeedItem;
import com.cascade.proto.feed.v1.GetFeedResponse;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.function.Function;
import java.util.stream.Collectors;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@Validated
@RestController
@RequestMapping("/api/feed")
public class FeedController {

  private final FeedClient feedClient;
  private final SocialGraphClient socialGraphClient;

  public FeedController(FeedClient feedClient, SocialGraphClient socialGraphClient) {
    this.feedClient = feedClient;
    this.socialGraphClient = socialGraphClient;
  }

  @GetMapping
  FeedResponse getFeed(
      @RequestHeader(value = UserIds.HEADER, required = false) String userIdHeader,
      @RequestParam(required = false) String pageToken,
      @RequestParam(defaultValue = "0") int pageSize) {
    long userId = UserIds.require(userIdHeader);
    GetFeedResponse feed = feedClient.getFeed(userId, pageToken, pageSize);
    Map<Long, UserView> authors = authors(feed.getItemsList());
    List<FeedItemView> items =
        feed.getItemsList().stream()
            .map(
                item -> {
                  UserView author = authors.get(item.getAuthorId());
                  return new FeedItemView(
                      item.getPostId(),
                      item.getAuthorId(),
                      item.getContent(),
                      item.getMediaUrl(),
                      item.getCreatedAtUnixMs(),
                      item.getRankScore(),
                      item.getRecencyScore(),
                      item.getEngagementScore(),
                      item.getAffinityScore(),
                      author == null ? null : AuthorView.from(author));
                })
            .toList();
    return new FeedResponse(items, feed.getNextPageToken());
  }

  private Map<Long, UserView> authors(List<FeedItem> items) {
    LinkedHashSet<Long> ids =
        items.stream()
            .map(FeedItem::getAuthorId)
            .filter(id -> id > 0)
            .collect(Collectors.toCollection(LinkedHashSet::new));
    if (ids.isEmpty()) {
      return Map.of();
    }
    return socialGraphClient.getUsers(ids).stream()
        .filter(Objects::nonNull)
        .collect(Collectors.toMap(UserView::id, Function.identity(), (left, right) -> left));
  }
}
