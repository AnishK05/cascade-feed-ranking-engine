package com.cascade.gateway.config;

import java.time.Duration;
import java.util.Arrays;
import java.util.List;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "cascade")
public record GatewayProperties(
    String postServiceAddr,
    String feedServiceAddr,
    String socialGraphBaseUrl,
    String corsAllowedOrigins,
    Duration grpcDeadline,
    String prometheusUrl) {

  public Duration grpcDeadlineOrDefault() {
    return grpcDeadline == null || grpcDeadline.isZero() || grpcDeadline.isNegative()
        ? Duration.ofSeconds(5)
        : grpcDeadline;
  }

  public List<String> allowedOrigins() {
    if (corsAllowedOrigins == null || corsAllowedOrigins.isBlank()) {
      return List.of();
    }
    return Arrays.stream(corsAllowedOrigins.split(","))
        .map(String::trim)
        .filter(origin -> !origin.isEmpty())
        .toList();
  }
}
