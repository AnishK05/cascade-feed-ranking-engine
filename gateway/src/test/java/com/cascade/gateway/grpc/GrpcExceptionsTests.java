package com.cascade.gateway.grpc;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;

import com.cascade.gateway.api.ForbiddenException;
import com.cascade.gateway.api.NotFoundException;
import com.cascade.gateway.api.UpstreamException;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.Test;

class GrpcExceptionsTests {

  @Test
  void mapsNotFoundAndPermissionDenied() {
    RuntimeException notFound =
        GrpcExceptions.map(new StatusRuntimeException(Status.NOT_FOUND.withDescription("gone")), "op");
    assertInstanceOf(NotFoundException.class, notFound);
    assertEquals("gone", notFound.getMessage());

    RuntimeException forbidden =
        GrpcExceptions.map(
            new StatusRuntimeException(Status.PERMISSION_DENIED.withDescription("nope")), "op");
    assertInstanceOf(ForbiddenException.class, forbidden);
  }

  @Test
  void mapsUnavailableToBadGateway() {
    RuntimeException unavailable =
        GrpcExceptions.map(new StatusRuntimeException(Status.UNAVAILABLE), "get feed");
    assertInstanceOf(UpstreamException.class, unavailable);
  }
}
