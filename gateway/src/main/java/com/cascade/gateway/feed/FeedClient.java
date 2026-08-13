package com.cascade.gateway.feed;

import com.cascade.proto.feed.v1.GetFeedResponse;

public interface FeedClient {

  GetFeedResponse getFeed(long userId, String pageToken, int pageSize);
}
