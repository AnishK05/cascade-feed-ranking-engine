package com.cascade.socialgraph.follow;

import static org.mockito.Mockito.doNothing;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.cascade.socialgraph.api.ApiExceptionHandler;
import com.cascade.socialgraph.user.UserResponse;
import java.time.OffsetDateTime;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.context.annotation.Import;
import org.springframework.http.MediaType;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(FollowController.class)
@Import(ApiExceptionHandler.class)
class FollowControllerTests {

  @Autowired private MockMvc mvc;
  @MockitoBean private FollowService followService;

  @Test
  void createsFollow() throws Exception {
    when(followService.follow(new FollowRequest(1, 2)))
        .thenReturn(new FollowResponse(1, 2, OffsetDateTime.now()));

    mvc.perform(
            post("/follows")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"followerId\":1,\"followeeId\":2}"))
        .andExpect(status().isCreated())
        .andExpect(header().string("Location", "/follows/1/2"))
        .andExpect(jsonPath("$.followeeId").value(2));
  }

  @Test
  void rejectsNonPositiveIds() throws Exception {
    mvc.perform(
            post("/follows")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"followerId\":0,\"followeeId\":2}"))
        .andExpect(status().isBadRequest());
  }

  @Test
  void deletesFollow() throws Exception {
    doNothing().when(followService).unfollow(1, 2);

    mvc.perform(delete("/follows/1/2")).andExpect(status().isNoContent());
  }

  @Test
  void returnsCursorPage() throws Exception {
    UserResponse user =
        new UserResponse(3, "follower", "Follower", false, 0, OffsetDateTime.now());
    when(followService.followers(2, "Mw", 1))
        .thenReturn(new CursorPage<>(List.of(user), "NA"));

    mvc.perform(get("/users/2/followers").param("cursor", "Mw").param("limit", "1"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.items[0].id").value(3))
        .andExpect(jsonPath("$.nextCursor").value("NA"));
  }

  @Test
  void rejectsOversizedPage() throws Exception {
    mvc.perform(get("/users/2/following").param("limit", "101"))
        .andExpect(status().isBadRequest());
  }
}
