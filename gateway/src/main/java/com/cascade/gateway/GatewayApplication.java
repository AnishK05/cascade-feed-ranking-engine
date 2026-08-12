package com.cascade.gateway;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * Entry point for the API Gateway / BFF. See IMPLEMENTATION_PLAN.md §5.5 for the service's
 * responsibilities: terminating HTTP/JSON from the frontend, translating to gRPC calls to
 * Post/Feed Service and REST calls to the Social Graph Service, and the auth stub.
 */
@SpringBootApplication
public class GatewayApplication {

  public static void main(String[] args) {
    SpringApplication.run(GatewayApplication.class, args);
  }
}
