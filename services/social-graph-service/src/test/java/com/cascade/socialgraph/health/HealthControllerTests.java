package com.cascade.socialgraph.health;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(HealthController.class)
class HealthControllerTests {

  @Autowired private MockMvc mvc;

  @Test
  void pingReturnsOkStatus() throws Exception {
    mvc.perform(get("/api/ping"))
        .andExpect(status().isOk())
        .andExpect(content().json("{\"service\":\"social-graph-service\",\"status\":\"ok\"}"));
  }
}
