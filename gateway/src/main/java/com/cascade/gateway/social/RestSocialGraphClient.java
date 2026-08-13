package com.cascade.gateway.social;

import com.cascade.gateway.api.BadRequestException;
import com.cascade.gateway.api.ConflictException;
import com.cascade.gateway.api.NotFoundException;
import com.cascade.gateway.api.UpstreamException;
import java.util.Arrays;
import java.util.Collection;
import java.util.List;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;
import org.springframework.web.client.RestClientException;
import org.springframework.web.client.RestClientResponseException;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;

@Component
public class RestSocialGraphClient implements SocialGraphClient {

  private static final ParameterizedTypeReference<CursorPage<UserView>> USER_PAGE =
      new ParameterizedTypeReference<>() {};

  private final RestClient restClient;
  private final ObjectMapper objectMapper;

  public RestSocialGraphClient(RestClient socialGraphRestClient, ObjectMapper objectMapper) {
    this.restClient = socialGraphRestClient;
    this.objectMapper = objectMapper;
  }

  @Override
  public UserView createUser(CreateUserRequest request) {
    try {
      return restClient
          .post()
          .uri("/users")
          .contentType(MediaType.APPLICATION_JSON)
          .body(request)
          .retrieve()
          .body(UserView.class);
    } catch (RestClientException exception) {
      throw map(exception, "create user");
    }
  }

  @Override
  public UserView getUser(long id) {
    try {
      return restClient.get().uri("/users/{id}", id).retrieve().body(UserView.class);
    } catch (RestClientException exception) {
      throw map(exception, "get user");
    }
  }

  @Override
  public List<UserView> getUsers(Collection<Long> ids) {
    if (ids == null || ids.isEmpty()) {
      return List.of();
    }
    try {
      UserView[] users =
          restClient
              .get()
              .uri(
                  uriBuilder ->
                      uriBuilder.path("/users").queryParam("ids", ids.toArray()).build())
              .retrieve()
              .body(UserView[].class);
      return users == null ? List.of() : Arrays.asList(users);
    } catch (RestClientException exception) {
      throw map(exception, "get users");
    }
  }

  @Override
  public List<UserView> listUsers(int limit) {
    try {
      UserView[] users =
          restClient
              .get()
              .uri(uriBuilder -> uriBuilder.path("/users").queryParam("limit", limit).build())
              .retrieve()
              .body(UserView[].class);
      return users == null ? List.of() : Arrays.asList(users);
    } catch (RestClientException exception) {
      throw map(exception, "list users");
    }
  }

  @Override
  public FollowView follow(long followerId, long followeeId) {
    try {
      return restClient
          .post()
          .uri("/follows")
          .contentType(MediaType.APPLICATION_JSON)
          .body(new FollowBody(followerId, followeeId))
          .retrieve()
          .body(FollowView.class);
    } catch (RestClientException exception) {
      throw map(exception, "follow");
    }
  }

  @Override
  public void unfollow(long followerId, long followeeId) {
    try {
      restClient
          .delete()
          .uri("/follows/{followerId}/{followeeId}", followerId, followeeId)
          .retrieve()
          .toBodilessEntity();
    } catch (RestClientException exception) {
      throw map(exception, "unfollow");
    }
  }

  @Override
  public CursorPage<UserView> followers(long userId, String cursor, int limit) {
    return page("/users/{id}/followers", userId, cursor, limit, "list followers");
  }

  @Override
  public CursorPage<UserView> following(long userId, String cursor, int limit) {
    return page("/users/{id}/following", userId, cursor, limit, "list following");
  }

  private CursorPage<UserView> page(
      String path, long userId, String cursor, int limit, String operation) {
    try {
      return restClient
          .get()
          .uri(
              uriBuilder -> {
                var builder = uriBuilder.path(path).queryParam("limit", limit);
                if (cursor != null && !cursor.isBlank()) {
                  builder.queryParam("cursor", cursor);
                }
                return builder.build(userId);
              })
          .retrieve()
          .body(USER_PAGE);
    } catch (RestClientException exception) {
      throw map(exception, operation);
    }
  }

  private RuntimeException map(RestClientException exception, String operation) {
    if (exception instanceof RestClientResponseException statusException) {
      String message = messageFrom(statusException, operation);
      return switch (statusException.getStatusCode().value()) {
        case 400 -> new BadRequestException(message);
        case 404 -> new NotFoundException(message);
        case 409 -> new ConflictException(message);
        default -> new UpstreamException(operation + " failed", exception);
      };
    }
    return new UpstreamException(operation + " failed", exception);
  }

  private String messageFrom(RestClientResponseException exception, String operation) {
    String body = exception.getResponseBodyAsString();
    if (body == null || body.isBlank()) {
      return operation + " failed";
    }
    try {
      JsonNode node = objectMapper.readTree(body);
      JsonNode message = node.get("message");
      if (message != null && message.isTextual() && !message.asText().isBlank()) {
        return message.asText();
      }
    } catch (Exception ignored) {
      // Fall through to the raw body or operation label.
    }
    return body.length() > 200 ? operation + " failed" : body;
  }

  private record FollowBody(long followerId, long followeeId) {}
}
