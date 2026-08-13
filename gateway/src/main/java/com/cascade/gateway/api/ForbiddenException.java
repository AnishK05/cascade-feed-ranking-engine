package com.cascade.gateway.api;

public class ForbiddenException extends RuntimeException {

  public ForbiddenException(String message) {
    super(message);
  }
}
