package com.cascade.gateway.config;

import com.cascade.proto.feed.v1.FeedServiceGrpc;
import com.cascade.proto.post.v1.PostServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class GrpcConfig {

  @Bean(name = "postChannel", destroyMethod = "shutdownNow")
  ManagedChannel postChannel(GatewayProperties properties) {
    return ManagedChannelBuilder.forTarget(properties.postServiceAddr()).usePlaintext().build();
  }

  @Bean(name = "feedChannel", destroyMethod = "shutdownNow")
  ManagedChannel feedChannel(GatewayProperties properties) {
    return ManagedChannelBuilder.forTarget(properties.feedServiceAddr()).usePlaintext().build();
  }

  @Bean
  PostServiceGrpc.PostServiceBlockingStub postStub(
      @Qualifier("postChannel") ManagedChannel postChannel) {
    return PostServiceGrpc.newBlockingStub(postChannel);
  }

  @Bean
  FeedServiceGrpc.FeedServiceBlockingStub feedStub(
      @Qualifier("feedChannel") ManagedChannel feedChannel) {
    return FeedServiceGrpc.newBlockingStub(feedChannel);
  }
}
