package com.cascade.gateway.auth;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.cascade.gateway.api.UnauthorizedException;
import org.junit.jupiter.api.Test;

class UserIdsTests {

  @Test
  void parsesPositiveId() {
    assertEquals(12, UserIds.require(" 12 "));
  }

  @Test
  void rejectsMissingAndInvalidValues() {
    assertThrows(UnauthorizedException.class, () -> UserIds.require(null));
    assertThrows(UnauthorizedException.class, () -> UserIds.require(""));
    assertThrows(UnauthorizedException.class, () -> UserIds.require("abc"));
    assertThrows(UnauthorizedException.class, () -> UserIds.require("0"));
  }
}
