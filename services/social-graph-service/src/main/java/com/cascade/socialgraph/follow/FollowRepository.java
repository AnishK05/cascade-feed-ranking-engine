package com.cascade.socialgraph.follow;

import com.cascade.socialgraph.user.User;
import java.util.List;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface FollowRepository extends JpaRepository<Follow, FollowId> {

  @Query(
      """
      select f.follower from Follow f
      where f.followee.id = :userId and f.follower.id > :cursor
      order by f.follower.id
      """)
  List<User> findFollowersAfter(
      @Param("userId") long userId, @Param("cursor") long cursor, Pageable pageable);

  @Query(
      """
      select f.followee from Follow f
      where f.follower.id = :userId and f.followee.id > :cursor
      order by f.followee.id
      """)
  List<User> findFollowingAfter(
      @Param("userId") long userId, @Param("cursor") long cursor, Pageable pageable);
}
