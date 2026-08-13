package com.cascade.gateway.admin;

import com.cascade.gateway.config.GatewayProperties;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.client.JdkClientHttpRequestFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;
import org.springframework.web.client.RestClientException;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;

@Component
public class RestPrometheusQueryClient implements PrometheusQueryClient {

  private static final Logger log = LoggerFactory.getLogger(RestPrometheusQueryClient.class);

  private static final String FEED_RPS =
      "sum(rate(feed_requests_total[1m]))";
  private static final String LATENCY_P50 =
      "histogram_quantile(0.50, sum by (le) (rate(feed_request_duration_seconds_bucket[5m])))";
  private static final String LATENCY_P95 =
      "histogram_quantile(0.95, sum by (le) (rate(feed_request_duration_seconds_bucket[5m])))";
  private static final String LATENCY_P99 =
      "histogram_quantile(0.99, sum by (le) (rate(feed_request_duration_seconds_bucket[5m])))";
  private static final String CACHE_HIT_RATIO =
      "sum(rate(feed_cache_hits_total[5m])) / (sum(rate(feed_cache_hits_total[5m])) + sum(rate(feed_cache_misses_total[5m])))";
  private static final String FANOUT_RPS =
      "sum(rate(fanout_events_processed_total[1m]))";
  private static final String FANOUT_LAG =
      "histogram_quantile(0.50, sum by (le) (rate(fanout_lag_ms_bucket[5m])))";
  private static final String KAFKA_LAG = "sum(kafka_consumer_lag)";

  private final RestClient restClient;
  private final ObjectMapper objectMapper;

  public RestPrometheusQueryClient(GatewayProperties properties, ObjectMapper objectMapper) {
    this.objectMapper = objectMapper;
    String base = properties.prometheusUrl();
    if (base == null || base.isBlank()) {
      base = "http://localhost:9095";
    }
    this.restClient =
        RestClient.builder()
            .baseUrl(base.replaceAll("/+$", ""))
            .requestFactory(new JdkClientHttpRequestFactory())
            .build();
  }

  @Override
  public AdminMetrics snapshot() {
    try {
      double hitsRatio = finiteOrZero(query(CACHE_HIT_RATIO));
      return new AdminMetrics(
          finiteOrZero(query(FEED_RPS)),
          new LatencyPercentiles(
              secondsToMillis(query(LATENCY_P50)),
              secondsToMillis(query(LATENCY_P95)),
              secondsToMillis(query(LATENCY_P99))),
          hitsRatio,
          finiteOrZero(query(FANOUT_RPS)),
          finiteOrZero(query(FANOUT_LAG)),
          finiteOrZero(query(KAFKA_LAG)),
          true);
    } catch (RuntimeException exception) {
      log.warn("prometheus query failed: {}", exception.toString());
      return new AdminMetrics(0, new LatencyPercentiles(0, 0, 0), 0, 0, 0, 0, false);
    }
  }

  private double query(String promql) {
    String body =
        restClient
            .get()
            .uri(uriBuilder -> uriBuilder.path("/api/v1/query").queryParam("query", promql).build())
            .retrieve()
            .body(String.class);
    if (body == null || body.isBlank()) {
      return Double.NaN;
    }
    try {
      JsonNode root = objectMapper.readTree(body);
      if ("error".equals(root.path("status").asText())) {
        String error = root.path("error").asText();
        throw new RestClientException(error.isBlank() ? "prometheus query failed" : error);
      }
      JsonNode result = root.path("data").path("result");
      if (!result.isArray() || result.size() == 0) {
        return Double.NaN;
      }
      JsonNode value = result.get(0).path("value");
      if (!value.isArray() || value.size() < 2) {
        return Double.NaN;
      }
      return Double.parseDouble(value.get(1).asText());
    } catch (RestClientException exception) {
      throw exception;
    } catch (Exception exception) {
      throw new RestClientException("decode prometheus response", exception);
    }
  }

  private static double secondsToMillis(double seconds) {
    return finiteOrZero(seconds) * 1000.0;
  }

  private static double finiteOrZero(double value) {
    return Double.isFinite(value) ? value : 0.0;
  }
}
