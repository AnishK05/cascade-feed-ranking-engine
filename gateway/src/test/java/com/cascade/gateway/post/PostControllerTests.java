package com.cascade.gateway.post;

import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.cascade.gateway.api.ApiExceptionHandler;
import com.cascade.gateway.api.ForbiddenException;
import com.cascade.gateway.auth.UserIds;
import com.cascade.proto.post.v1.CreatePostResponse;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.context.annotation.Import;
import org.springframework.http.MediaType;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(PostController.class)
@Import(ApiExceptionHandler.class)
class PostControllerTests {

  @Autowired private MockMvc mvc;
  @MockitoBean private PostClient postClient;

  @Test
  void createsPostAsHeaderUser() throws Exception {
    when(postClient.create(9, "hello", ""))
        .thenReturn(CreatePostResponse.newBuilder().setPostId(44).setCreatedAtUnixMs(123).build());

    mvc.perform(
            post("/api/posts")
                .header(UserIds.HEADER, "9")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"content\":\"hello\"}"))
        .andExpect(status().isCreated())
        .andExpect(jsonPath("$.postId").value(44))
        .andExpect(jsonPath("$.authorId").value(9));
  }

  @Test
  void createRequiresUserId() throws Exception {
    mvc.perform(
            post("/api/posts")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"content\":\"hello\"}"))
        .andExpect(status().isUnauthorized());
  }

  @Test
  void mapsForbiddenDelete() throws Exception {
    org.mockito.Mockito.doThrow(new ForbiddenException("requesting user is not the post author"))
        .when(postClient)
        .delete(5, 9);

    mvc.perform(delete("/api/posts/5").header(UserIds.HEADER, "9"))
        .andExpect(status().isForbidden())
        .andExpect(jsonPath("$.message").value("requesting user is not the post author"));
  }
}
