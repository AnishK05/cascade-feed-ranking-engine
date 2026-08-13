package com.cascade.gateway.social;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Size;

public record CreateUserRequest(
    @NotBlank
        @Size(max = 50)
        @Pattern(
            regexp = "[A-Za-z0-9_]+",
            message = "must contain only letters, numbers, and underscores")
        String username,
    @NotBlank @Size(max = 100) String displayName) {}
