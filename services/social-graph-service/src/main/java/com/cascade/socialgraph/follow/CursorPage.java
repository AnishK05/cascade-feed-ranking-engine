package com.cascade.socialgraph.follow;

import java.util.List;

public record CursorPage<T>(List<T> items, String nextCursor) {}
