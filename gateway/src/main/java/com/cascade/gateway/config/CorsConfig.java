package com.cascade.gateway.config;

import java.util.List;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.servlet.config.annotation.CorsRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

@Configuration
public class CorsConfig implements WebMvcConfigurer {

  private final GatewayProperties properties;

  public CorsConfig(GatewayProperties properties) {
    this.properties = properties;
  }

  @Override
  public void addCorsMappings(CorsRegistry registry) {
    List<String> origins = properties.allowedOrigins();
    if (origins == null || origins.isEmpty()) {
      return;
    }
    registry
        .addMapping("/api/**")
        .allowedOrigins(origins.toArray(String[]::new))
        .allowedMethods("GET", "POST", "DELETE", "OPTIONS")
        .allowedHeaders("*")
        .exposedHeaders("Location");
  }
}
