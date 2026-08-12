package com.cascade.gateway.health;

import java.util.Map;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * A tiny, dependency-free liveness endpoint distinct from Actuator's `/actuator/health`, so the
 * service has at least one hand-written REST endpoint from Phase 0 onward. Real endpoints are
 * added starting in Phase 9 (Gateway/BFF).
 */
@RestController
public class HealthController {

  @GetMapping("/api/ping")
  public Map<String, String> ping() {
    return Map.of("service", "gateway", "status", "ok");
  }
}
