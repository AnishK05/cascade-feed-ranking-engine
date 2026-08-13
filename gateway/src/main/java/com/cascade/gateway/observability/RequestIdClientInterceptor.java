package com.cascade.gateway.observability;

import io.grpc.CallOptions;
import io.grpc.Channel;
import io.grpc.ClientCall;
import io.grpc.ClientInterceptor;
import io.grpc.ForwardingClientCall.SimpleForwardingClientCall;
import io.grpc.Metadata;
import io.grpc.MethodDescriptor;

public class RequestIdClientInterceptor implements ClientInterceptor {

  private static final Metadata.Key<String> KEY =
      Metadata.Key.of(RequestIds.GRPC_METADATA, Metadata.ASCII_STRING_MARSHALLER);

  @Override
  public <ReqT, RespT> ClientCall<ReqT, RespT> interceptCall(
      MethodDescriptor<ReqT, RespT> method, CallOptions callOptions, Channel next) {
    return new SimpleForwardingClientCall<>(next.newCall(method, callOptions)) {
      @Override
      public void start(Listener<RespT> responseListener, Metadata headers) {
        String requestId = RequestIds.current();
        if (requestId != null && !requestId.isBlank()) {
          headers.put(KEY, requestId);
        }
        super.start(responseListener, headers);
      }
    };
  }
}
