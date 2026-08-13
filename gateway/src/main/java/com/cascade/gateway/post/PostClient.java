package com.cascade.gateway.post;

import com.cascade.proto.post.v1.CreatePostResponse;
import com.cascade.proto.post.v1.Post;
import java.util.List;

public interface PostClient {

  CreatePostResponse create(long authorId, String content, String mediaUrl);

  List<Post> getPosts(List<Long> ids);

  void delete(long postId, long requestingUserId);
}
