package com.cascade.socialgraph;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * Entry point for the Social Graph Service. Owns the `users` and `follows` tables and exposes
 * REST endpoints for user CRUD and follow/unfollow. See IMPLEMENTATION_PLAN.md §5.2.
 */
@SpringBootApplication
public class SocialGraphServiceApplication {

  public static void main(String[] args) {
    SpringApplication.run(SocialGraphServiceApplication.class, args);
  }
}
