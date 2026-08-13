package com.cascade.socialgraph.user;

import jakarta.validation.Valid;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.Positive;
import java.net.URI;
import java.util.List;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.annotation.Validated;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@Validated
@RestController
@RequestMapping("/users")
public class UserController {

  private final UserService userService;

  public UserController(UserService userService) {
    this.userService = userService;
  }

  @PostMapping
  ResponseEntity<UserResponse> create(@Valid @RequestBody CreateUserRequest request) {
    UserResponse response = userService.create(request);
    return ResponseEntity.created(URI.create("/users/" + response.id())).body(response);
  }

  @GetMapping("/{id}")
  UserResponse get(@PathVariable @Positive long id) {
    return userService.get(id);
  }

  @GetMapping(params = "ids")
  List<UserResponse> getMany(@RequestParam List<Long> ids) {
    return userService.getMany(ids);
  }

  @GetMapping(params = "!ids")
  List<UserResponse> list(
      @RequestParam(defaultValue = "50") @Min(1) @Max(100) int limit) {
    return userService.list(limit);
  }
}
