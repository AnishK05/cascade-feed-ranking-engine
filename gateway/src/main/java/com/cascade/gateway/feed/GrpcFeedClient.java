package com.cascade.gateway.feed;

import com.cascade.gateway.config.GatewayProperties;
import com.cascade.gateway.grpc.GrpcExceptions;
import com.cascade.proto.feed.v1.FeedServiceGrpc;
import com.cascade.proto.feed.v1.GetFeedRequest;
import com.cascade.proto.feed.v1.GetFeedResponse;
import io.grpc.StatusRuntimeException;
import java.util.concurrent.TimeUnit;
import org.springframework.stereotype.Component;

@Component
public class GrpcFeedClient implements FeedClient {

  private final FeedServiceGrpc.FeedServiceBlockingStub stub;
  private final long deadlineMs;

  public GrpcFeedClient(
      FeedServiceGrpc.FeedServiceBlockingStub stub, GatewayProperties properties) {
    this.stub = stub;
    this.deadlineMs = properties.grpcDeadlineOrDefault().toMillis();
  }

  @Override
  public GetFeedResponse getFeed(long userId, String pageToken, int pageSize) {
    try {
      GetFeedRequest.Builder builder = GetFeedRequest.newBuilder().setUserId(userId);
      if (pageToken != null && !pageToken.isBlank()) {
        builder.setPageToken(pageToken);
      }
      if (pageSize > 0) {
        builder.setPageSize(pageSize);
      }
      return stub.withDeadlineAfter(deadlineMs, TimeUnit.MILLISECONDS).getFeed(builder.build());
    } catch (StatusRuntimeException exception) {
      throw GrpcExceptions.map(exception, "get feed");
    }
  }
}
