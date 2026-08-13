package com.cascade.gateway.social;

import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.cascade.gateway.api.ApiExceptionHandler;
import com.cascade.gateway.auth.UserIds;
import java.time.OffsetDateTime;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.context.annotation.Import;
import org.springframework.http.MediaType;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(SocialGraphController.class)
@Import(ApiExceptionHandler.class)
class SocialGraphControllerTests {

  @Autowired private MockMvc mvc;
  @MockitoBean private SocialGraphClient socialGraphClient;

  @Test
  void createsUserWithoutAuthHeader() throws Exception {
    when(socialGraphClient.createUser(new CreateUserRequest("alice", "Alice")))
        .thenReturn(new UserView(3, "alice", "Alice", false, 0, OffsetDateTime.now()));

    mvc.perform(
            post("/api/users")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"username\":\"alice\",\"displayName\":\"Alice\"}"))
        .andExpect(status().isCreated())
        .andExpect(header().string("Location", "/api/users/3"))
        .andExpect(jsonPath("$.username").value("alice"));
  }

  @Test
  void followUsesHeaderAsFollower() throws Exception {
    when(socialGraphClient.follow(7, 2))
        .thenReturn(new FollowView(7, 2, OffsetDateTime.now()));

    mvc.perform(
            post("/api/follows")
                .header(UserIds.HEADER, "7")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"followeeId\":2}"))
        .andExpect(status().isCreated())
        .andExpect(header().string("Location", "/api/follows/2"))
        .andExpect(jsonPath("$.followerId").value(7));
  }

  @Test
  void unfollowRequiresUserId() throws Exception {
    mvc.perform(delete("/api/follows/2")).andExpect(status().isUnauthorized());
  }

  @Test
  void listsFollowers() throws Exception {
    when(socialGraphClient.followers(2, null, 20))
        .thenReturn(
            new CursorPage<>(
                List.of(new UserView(7, "ann", "Ann", false, 0, OffsetDateTime.now())), "c2"));

    mvc.perform(get("/api/users/2/followers"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.items[0].id").value(7))
        .andExpect(jsonPath("$.nextCursor").value("c2"));

    verify(socialGraphClient).followers(2, null, 20);
  }
}
