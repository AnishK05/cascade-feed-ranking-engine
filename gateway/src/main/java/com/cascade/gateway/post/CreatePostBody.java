package com.cascade.gateway.post;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record CreatePostBody(
    @NotBlank @Size(max = 5000) String content, @Size(max = 2048) String mediaUrl) {}
