package com.cascade.gateway.admin;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/admin")
public class AdminController {

  private final PrometheusQueryClient prometheusQueryClient;

  public AdminController(PrometheusQueryClient prometheusQueryClient) {
    this.prometheusQueryClient = prometheusQueryClient;
  }

  @GetMapping("/metrics")
  AdminMetrics metrics() {
    return prometheusQueryClient.snapshot();
  }
}
