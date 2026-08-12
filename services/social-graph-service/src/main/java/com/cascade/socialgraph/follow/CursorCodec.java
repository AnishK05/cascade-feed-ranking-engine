package com.cascade.socialgraph.follow;

import com.cascade.socialgraph.api.BadRequestException;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;

@Component
public class CursorCodec {

  long decode(String cursor) {
    if (!StringUtils.hasText(cursor)) {
      return 0;
    }
    try {
      String value =
          new String(Base64.getUrlDecoder().decode(cursor), StandardCharsets.UTF_8);
      long id = Long.parseLong(value);
      if (id < 0) {
        throw new NumberFormatException("negative cursor");
      }
      return id;
    } catch (IllegalArgumentException exception) {
      throw new BadRequestException("Invalid cursor");
    }
  }

  String encode(long id) {
    return Base64.getUrlEncoder()
        .withoutPadding()
        .encodeToString(Long.toString(id).getBytes(StandardCharsets.UTF_8));
  }
}
