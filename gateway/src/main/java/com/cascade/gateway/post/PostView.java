package com.cascade.gateway.post;

public record PostView(
    long id, long authorId, String content, String mediaUrl, long createdAtUnixMs) {}
