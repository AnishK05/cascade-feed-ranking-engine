package com.cascade.socialgraph.user;

import java.util.List;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/internal/celebrities")
public class InternalCelebrityController {

  private final UserService userService;

  public InternalCelebrityController(UserService userService) {
    this.userService = userService;
  }

  @GetMapping
  List<UserResponse> celebrities() {
    return userService.celebrities();
  }
}
