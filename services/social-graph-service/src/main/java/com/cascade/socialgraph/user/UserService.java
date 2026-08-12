package com.cascade.socialgraph.user;

import com.cascade.socialgraph.api.ConflictException;
import com.cascade.socialgraph.api.NotFoundException;
import java.util.List;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class UserService {

  private final UserRepository userRepository;

  public UserService(UserRepository userRepository) {
    this.userRepository = userRepository;
  }

  @Transactional
  public UserResponse create(CreateUserRequest request) {
    if (userRepository.existsByUsername(request.username())) {
      throw new ConflictException("Username already exists: " + request.username());
    }
    try {
      return UserResponse.from(
          userRepository.saveAndFlush(new User(request.username(), request.displayName())));
    } catch (DataIntegrityViolationException exception) {
      throw new ConflictException("Username already exists: " + request.username());
    }
  }

  @Transactional(readOnly = true)
  public UserResponse get(long id) {
    return UserResponse.from(requireUser(id));
  }

  @Transactional(readOnly = true)
  public List<UserResponse> celebrities() {
    return userRepository.findByCelebrityTrueOrderByIdAsc().stream()
        .map(UserResponse::from)
        .toList();
  }

  private User requireUser(long id) {
    return userRepository
        .findById(id)
        .orElseThrow(() -> new NotFoundException("User not found: " + id));
  }
}
