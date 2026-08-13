package com.cascade.gateway.feed;

import java.util.List;

public record FeedResponse(List<FeedItemView> items, String nextPageToken) {}
