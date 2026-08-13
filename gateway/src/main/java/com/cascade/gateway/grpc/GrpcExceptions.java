package com.cascade.gateway.grpc;

import com.cascade.gateway.api.BadRequestException;
import com.cascade.gateway.api.ForbiddenException;
import com.cascade.gateway.api.NotFoundException;
import com.cascade.gateway.api.UpstreamException;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;

public final class GrpcExceptions {

  private GrpcExceptions() {}

  public static RuntimeException map(StatusRuntimeException exception, String operation) {
    String description = exception.getStatus().getDescription();
    if (description == null || description.isBlank()) {
      description = operation + " failed";
    }
    Status.Code code = exception.getStatus().getCode();
    return switch (code) {
      case NOT_FOUND -> new NotFoundException(description);
      case INVALID_ARGUMENT -> new BadRequestException(description);
      case PERMISSION_DENIED -> new ForbiddenException(description);
      case UNAVAILABLE, DEADLINE_EXCEEDED ->
          new UpstreamException(operation + " is unavailable", exception);
      default -> new UpstreamException(operation + " failed", exception);
    };
  }
}
