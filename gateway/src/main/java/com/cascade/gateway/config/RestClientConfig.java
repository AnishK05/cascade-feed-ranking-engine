package com.cascade.gateway.config;

import com.cascade.gateway.observability.RequestIds;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.client.JdkClientHttpRequestFactory;
import org.springframework.web.client.RestClient;

@Configuration
public class RestClientConfig {

  @Bean
  RestClient socialGraphRestClient(GatewayProperties properties) {
    return RestClient.builder()
        .baseUrl(properties.socialGraphBaseUrl())
        .requestFactory(new JdkClientHttpRequestFactory())
        .requestInterceptor(
            (request, body, execution) -> {
              String requestId = RequestIds.current();
              if (requestId != null && !requestId.isBlank()) {
                request.getHeaders().set(RequestIds.HEADER, requestId);
              }
              return execution.execute(request, body);
            })
        .build();
  }
}
