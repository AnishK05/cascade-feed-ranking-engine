package com.cascade.gateway.api;

import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.ConstraintViolationException;
import java.time.OffsetDateTime;
import java.util.List;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import org.springframework.web.method.annotation.HandlerMethodValidationException;
import org.springframework.web.method.annotation.MethodArgumentTypeMismatchException;

@RestControllerAdvice
public class ApiExceptionHandler {

  @ExceptionHandler(UnauthorizedException.class)
  ResponseEntity<ApiError> unauthorized(UnauthorizedException exception, HttpServletRequest request) {
    return error(HttpStatus.UNAUTHORIZED, exception.getMessage(), List.of(), request);
  }

  @ExceptionHandler(ForbiddenException.class)
  ResponseEntity<ApiError> forbidden(ForbiddenException exception, HttpServletRequest request) {
    return error(HttpStatus.FORBIDDEN, exception.getMessage(), List.of(), request);
  }

  @ExceptionHandler(NotFoundException.class)
  ResponseEntity<ApiError> notFound(NotFoundException exception, HttpServletRequest request) {
    return error(HttpStatus.NOT_FOUND, exception.getMessage(), List.of(), request);
  }

  @ExceptionHandler(ConflictException.class)
  ResponseEntity<ApiError> conflict(ConflictException exception, HttpServletRequest request) {
    return error(HttpStatus.CONFLICT, exception.getMessage(), List.of(), request);
  }

  @ExceptionHandler(UpstreamException.class)
  ResponseEntity<ApiError> upstream(UpstreamException exception, HttpServletRequest request) {
    return error(HttpStatus.BAD_GATEWAY, exception.getMessage(), List.of(), request);
  }

  @ExceptionHandler({
    BadRequestException.class,
    HttpMessageNotReadableException.class,
    ConstraintViolationException.class,
    MethodArgumentTypeMismatchException.class
  })
  ResponseEntity<ApiError> badRequest(Exception exception, HttpServletRequest request) {
    return error(HttpStatus.BAD_REQUEST, exception.getMessage(), List.of(), request);
  }

  @ExceptionHandler(MethodArgumentNotValidException.class)
  ResponseEntity<ApiError> invalidBody(
      MethodArgumentNotValidException exception, HttpServletRequest request) {
    List<String> details =
        exception.getBindingResult().getFieldErrors().stream()
            .map(error -> error.getField() + ": " + error.getDefaultMessage())
            .toList();
    return error(HttpStatus.BAD_REQUEST, "Request validation failed", details, request);
  }

  @ExceptionHandler(HandlerMethodValidationException.class)
  ResponseEntity<ApiError> invalidParameter(
      HandlerMethodValidationException exception, HttpServletRequest request) {
    return error(
        HttpStatus.BAD_REQUEST, "Request parameter validation failed", List.of(), request);
  }

  private ResponseEntity<ApiError> error(
      HttpStatus status, String message, List<String> details, HttpServletRequest request) {
    return ResponseEntity.status(status)
        .body(
            new ApiError(
                OffsetDateTime.now(),
                status.value(),
                status.getReasonPhrase(),
                message,
                details,
                request.getRequestURI()));
  }

  record ApiError(
      OffsetDateTime timestamp,
      int status,
      String error,
      String message,
      List<String> details,
      String path) {}
}
