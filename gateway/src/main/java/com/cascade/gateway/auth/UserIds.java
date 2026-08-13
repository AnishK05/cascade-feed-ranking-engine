package com.cascade.gateway.auth;

import com.cascade.gateway.api.UnauthorizedException;

/**
 * Auth stub for the demo user-switcher. The {@code X-User-Id} header is deliberately spoofable
 * and is not a security boundary (IMPLEMENTATION_PLAN.md §5.5, §17).
 */
public final class UserIds {

  public static final String HEADER = "X-User-Id";

  private UserIds() {}

  public static long require(String header) {
    if (header == null || header.isBlank()) {
      throw new UnauthorizedException("X-User-Id header is required");
    }
    try {
      long userId = Long.parseLong(header.trim());
      if (userId <= 0) {
        throw new UnauthorizedException("X-User-Id must be a positive user ID");
      }
      return userId;
    } catch (NumberFormatException exception) {
      throw new UnauthorizedException("X-User-Id must be a positive user ID");
    }
  }
}
