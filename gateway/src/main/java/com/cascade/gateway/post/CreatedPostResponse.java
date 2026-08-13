package com.cascade.gateway.post;

public record CreatedPostResponse(long postId, long authorId, long createdAtUnixMs) {}
