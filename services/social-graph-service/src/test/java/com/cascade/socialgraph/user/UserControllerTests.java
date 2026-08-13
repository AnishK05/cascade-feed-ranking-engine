package com.cascade.socialgraph.user;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.cascade.socialgraph.api.ApiExceptionHandler;
import com.cascade.socialgraph.api.ConflictException;
import com.cascade.socialgraph.api.NotFoundException;
import java.time.OffsetDateTime;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.context.annotation.Import;
import org.springframework.http.MediaType;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest({UserController.class, InternalCelebrityController.class})
@Import(ApiExceptionHandler.class)
class UserControllerTests {

  @Autowired private MockMvc mvc;
  @MockitoBean private UserService userService;

  @Test
  void createsUser() throws Exception {
    when(userService.create(any()))
        .thenReturn(new UserResponse(12, "alice", "Alice", false, 0, OffsetDateTime.now()));

    mvc.perform(
            post("/users")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"username\":\"alice\",\"displayName\":\"Alice\"}"))
        .andExpect(status().isCreated())
        .andExpect(header().string("Location", "/users/12"))
        .andExpect(jsonPath("$.username").value("alice"));
  }

  @Test
  void rejectsInvalidUserBody() throws Exception {
    mvc.perform(
            post("/users")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"username\":\"bad name\",\"displayName\":\"\"}"))
        .andExpect(status().isBadRequest())
        .andExpect(jsonPath("$.message").value("Request validation failed"));
  }

  @Test
  void mapsMissingUserTo404() throws Exception {
    when(userService.get(99)).thenThrow(new NotFoundException("User not found: 99"));

    mvc.perform(get("/users/99"))
        .andExpect(status().isNotFound())
        .andExpect(jsonPath("$.message").value("User not found: 99"));
  }

  @Test
  void returnsUsersByIds() throws Exception {
    when(userService.getMany(List.of(1L, 2L)))
        .thenReturn(
            List.of(
                new UserResponse(1, "alice", "Alice", false, 0, OffsetDateTime.now()),
                new UserResponse(2, "bob", "Bob", true, 12, OffsetDateTime.now())));

    mvc.perform(get("/users").param("ids", "1,2"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$[0].username").value("alice"))
        .andExpect(jsonPath("$[1].id").value(2));
  }

  @Test
  void listsUsersWhenIdsOmitted() throws Exception {
    when(userService.list(50))
        .thenReturn(List.of(new UserResponse(1, "alice", "Alice", false, 0, OffsetDateTime.now())));

    mvc.perform(get("/users"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$[0].username").value("alice"));
  }

  @Test
  void mapsDuplicateUsernameTo409() throws Exception {
    when(userService.create(any())).thenThrow(new ConflictException("Username already exists"));

    mvc.perform(
            post("/users")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"username\":\"alice\",\"displayName\":\"Alice\"}"))
        .andExpect(status().isConflict())
        .andExpect(jsonPath("$.status").value(409));
  }
}
