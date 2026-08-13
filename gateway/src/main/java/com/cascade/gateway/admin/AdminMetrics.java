package com.cascade.gateway.admin;

public record AdminMetrics(
    double requestsPerSecond,
    LatencyPercentiles feedLatencyMs,
    double cacheHitRatio,
    double fanoutEventsPerSecond,
    double fanoutLagMs,
    double kafkaConsumerLag,
    boolean available) {}
