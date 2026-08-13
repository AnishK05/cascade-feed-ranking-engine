package com.cascade.gateway.observability;

/**
 * Process-wide current request ID. Gateway sets this from {@code X-Request-Id} (or generates
 * one) so gRPC and REST client interceptors can attach it without changing every call site.
 */
public final class RequestIds {

  public static final String HEADER = "X-Request-Id";
  public static final String MDC_KEY = "request_id";
  public static final String GRPC_METADATA = "x-request-id";

  private static final ThreadLocal<String> CURRENT = new ThreadLocal<>();

  private RequestIds() {}

  public static void set(String id) {
    CURRENT.set(id);
  }

  public static String current() {
    return CURRENT.get();
  }

  public static void clear() {
    CURRENT.remove();
  }
}
