package com.cascade.gateway.admin;

import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.cascade.gateway.api.ApiExceptionHandler;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.context.annotation.Import;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(AdminController.class)
@Import(ApiExceptionHandler.class)
class AdminControllerTests {

  @Autowired private MockMvc mvc;
  @MockitoBean private PrometheusQueryClient prometheusQueryClient;

  @Test
  void returnsLiveSnapshot() throws Exception {
    when(prometheusQueryClient.snapshot())
        .thenReturn(
            new AdminMetrics(12.5, new LatencyPercentiles(8, 20, 40), 0.9, 3.2, 15, 4, true));

    mvc.perform(get("/api/admin/metrics"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.requestsPerSecond").value(12.5))
        .andExpect(jsonPath("$.feedLatencyMs.p95").value(20.0))
        .andExpect(jsonPath("$.cacheHitRatio").value(0.9))
        .andExpect(jsonPath("$.available").value(true));
  }
}
