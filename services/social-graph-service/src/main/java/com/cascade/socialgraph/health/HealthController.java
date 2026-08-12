package com.cascade.socialgraph.health;

import java.util.Map;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * A tiny, dependency-free liveness endpoint distinct from Actuator's `/actuator/health`. Real
 * user/follow endpoints are added starting in Phase 2.
 */
@RestController
public class HealthController {

  @GetMapping("/api/ping")
  public Map<String, String> ping() {
    return Map.of("service", "social-graph-service", "status", "ok");
  }
}
