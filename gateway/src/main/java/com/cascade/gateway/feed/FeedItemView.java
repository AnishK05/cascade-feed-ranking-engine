package com.cascade.gateway.feed;

public record FeedItemView(
    long postId,
    long authorId,
    String content,
    String mediaUrl,
    long createdAtUnixMs,
    double rankScore,
    AuthorView author) {}
