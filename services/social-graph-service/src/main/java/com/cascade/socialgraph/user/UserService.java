package com.cascade.socialgraph.user;

import com.cascade.socialgraph.api.BadRequestException;
import com.cascade.socialgraph.api.ConflictException;
import com.cascade.socialgraph.api.NotFoundException;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.data.domain.PageRequest;
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
  public List<UserResponse> getMany(List<Long> ids) {
    if (ids == null || ids.isEmpty()) {
      return List.of();
    }
    if (ids.size() > 100) {
      throw new BadRequestException("ids must contain at most 100 values");
    }
    LinkedHashSet<Long> unique = new LinkedHashSet<>();
    for (Long id : ids) {
      if (id == null || id <= 0) {
        throw new BadRequestException("ids must contain only positive IDs");
      }
      unique.add(id);
    }
    Map<Long, User> found =
        userRepository.findAllById(unique).stream()
            .collect(Collectors.toMap(User::getId, Function.identity()));
    List<UserResponse> result = new ArrayList<>();
    for (Long id : unique) {
      User user = found.get(id);
      if (user != null) {
        result.add(UserResponse.from(user));
      }
    }
    return result;
  }

  @Transactional(readOnly = true)
  public List<UserResponse> celebrities() {
    return userRepository.findByCelebrityTrueOrderByIdAsc().stream()
        .map(UserResponse::from)
        .toList();
  }

  @Transactional(readOnly = true)
  public List<UserResponse> list(int limit) {
    if (limit < 1 || limit > 100) {
      throw new BadRequestException("limit must be between 1 and 100");
    }
    return userRepository.findAllByOrderByIdAsc(PageRequest.of(0, limit)).stream()
        .map(UserResponse::from)
        .toList();
  }

  private User requireUser(long id) {
    return userRepository
        .findById(id)
        .orElseThrow(() -> new NotFoundException("User not found: " + id));
  }
}
