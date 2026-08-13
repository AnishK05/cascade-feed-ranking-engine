package com.cascade.gateway.api;

public class UpstreamException extends RuntimeException {

  public UpstreamException(String message, Throwable cause) {
    super(message, cause);
  }

  public UpstreamException(String message) {
    super(message);
  }
}
