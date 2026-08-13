package com.cascade.gateway.feed;

import static org.mockito.ArgumentMatchers.anyCollection;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.cascade.gateway.api.ApiExceptionHandler;
import com.cascade.gateway.auth.UserIds;
import com.cascade.gateway.social.SocialGraphClient;
import com.cascade.gateway.social.UserView;
import com.cascade.proto.feed.v1.FeedItem;
import com.cascade.proto.feed.v1.GetFeedResponse;
import java.time.OffsetDateTime;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest;
import org.springframework.context.annotation.Import;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(FeedController.class)
@Import(ApiExceptionHandler.class)
class FeedControllerTests {

  @Autowired private MockMvc mvc;
  @MockitoBean private FeedClient feedClient;
  @MockitoBean private SocialGraphClient socialGraphClient;

  @Test
  void requiresUserIdHeader() throws Exception {
    mvc.perform(get("/api/feed")).andExpect(status().isUnauthorized());
  }

  @Test
  void aggregatesAuthorProfilesForTheAuthenticatedUser() throws Exception {
    when(feedClient.getFeed(7, null, 0))
        .thenReturn(
            GetFeedResponse.newBuilder()
                .addItems(
                    FeedItem.newBuilder()
                        .setPostId(11)
                        .setAuthorId(2)
                        .setContent("hello")
                        .setCreatedAtUnixMs(100)
                        .setRankScore(1.5)
                        .build())
                .setNextPageToken("next")
                .build());
    when(socialGraphClient.getUsers(anyCollection()))
        .thenReturn(List.of(new UserView(2, "bob", "Bob", true, 12, OffsetDateTime.now())));

    mvc.perform(get("/api/feed").header(UserIds.HEADER, "7"))
        .andExpect(status().isOk())
        .andExpect(jsonPath("$.items[0].postId").value(11))
        .andExpect(jsonPath("$.items[0].author.username").value("bob"))
        .andExpect(jsonPath("$.items[0].author.celebrity").value(true))
        .andExpect(jsonPath("$.nextPageToken").value("next"));

    verify(feedClient).getFeed(7, null, 0);
  }

  @Test
  void rejectsNonPositiveUserId() throws Exception {
    mvc.perform(get("/api/feed").header(UserIds.HEADER, "0"))
        .andExpect(status().isUnauthorized());
  }
}
