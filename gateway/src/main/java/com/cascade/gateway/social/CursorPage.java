package com.cascade.gateway.social;

import java.util.List;

public record CursorPage<T>(List<T> items, String nextCursor) {}
