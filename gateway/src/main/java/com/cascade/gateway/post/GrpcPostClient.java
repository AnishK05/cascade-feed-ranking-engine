package com.cascade.gateway.post;

import com.cascade.gateway.config.GatewayProperties;
import com.cascade.gateway.grpc.GrpcExceptions;
import com.cascade.proto.post.v1.CreatePostRequest;
import com.cascade.proto.post.v1.CreatePostResponse;
import com.cascade.proto.post.v1.DeletePostRequest;
import com.cascade.proto.post.v1.GetPostsRequest;
import com.cascade.proto.post.v1.Post;
import com.cascade.proto.post.v1.PostServiceGrpc;
import io.grpc.StatusRuntimeException;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.springframework.stereotype.Component;

@Component
public class GrpcPostClient implements PostClient {

  private final PostServiceGrpc.PostServiceBlockingStub stub;
  private final long deadlineMs;

  public GrpcPostClient(
      PostServiceGrpc.PostServiceBlockingStub stub, GatewayProperties properties) {
    this.stub = stub;
    this.deadlineMs = properties.grpcDeadlineOrDefault().toMillis();
  }

  @Override
  public CreatePostResponse create(long authorId, String content, String mediaUrl) {
    try {
      return stub.withDeadlineAfter(deadlineMs, TimeUnit.MILLISECONDS)
          .createPost(
              CreatePostRequest.newBuilder()
                  .setAuthorId(authorId)
                  .setContent(content)
                  .setMediaUrl(mediaUrl == null ? "" : mediaUrl)
                  .build());
    } catch (StatusRuntimeException exception) {
      throw GrpcExceptions.map(exception, "create post");
    }
  }

  @Override
  public List<Post> getPosts(List<Long> ids) {
    try {
      return stub.withDeadlineAfter(deadlineMs, TimeUnit.MILLISECONDS)
          .getPosts(GetPostsRequest.newBuilder().addAllPostIds(ids).build())
          .getPostsList();
    } catch (StatusRuntimeException exception) {
      throw GrpcExceptions.map(exception, "get posts");
    }
  }

  @Override
  public void delete(long postId, long requestingUserId) {
    try {
      stub.withDeadlineAfter(deadlineMs, TimeUnit.MILLISECONDS)
          .deletePost(
              DeletePostRequest.newBuilder()
                  .setPostId(postId)
                  .setRequestingUserId(requestingUserId)
                  .build());
    } catch (StatusRuntimeException exception) {
      throw GrpcExceptions.map(exception, "delete post");
    }
  }
}
