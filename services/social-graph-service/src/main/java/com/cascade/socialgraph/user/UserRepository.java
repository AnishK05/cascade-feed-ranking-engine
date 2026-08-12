package com.cascade.socialgraph.user;

import jakarta.persistence.LockModeType;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface UserRepository extends JpaRepository<User, Long> {

  boolean existsByUsername(String username);

  @Lock(LockModeType.PESSIMISTIC_WRITE)
  @Query("select u from User u where u.id = :id")
  Optional<User> findByIdForUpdate(@Param("id") long id);

  List<User> findByCelebrityTrueOrderByIdAsc();
}
