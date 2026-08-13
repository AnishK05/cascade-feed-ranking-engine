import type { AdminMetrics, CreatedPost, CursorPage, FeedPage, User } from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "http://localhost:8080";

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(
  path: string,
  init: RequestInit & { userId?: number } = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (init.userId && init.userId > 0) {
    headers.set("X-User-Id", String(init.userId));
  }
  const response = await fetch(`${API_BASE}${path}`, { ...init, headers });
  if (response.status === 204) {
    return undefined as T;
  }
  const text = await response.text();
  const payload = text ? (JSON.parse(text) as { message?: string } & T) : undefined;
  if (!response.ok) {
    throw new ApiError(response.status, payload && "message" in payload && payload.message
      ? payload.message
      : `Request failed (${response.status})`);
  }
  return payload as T;
}

export const api = {
  listUsers(limit = 100) {
    return request<User[]>(`/api/users?limit=${limit}`);
  },
  getUser(id: number) {
    return request<User>(`/api/users/${id}`);
  },
  getFeed(userId: number, pageToken = "", pageSize = 20) {
    const params = new URLSearchParams({ pageSize: String(pageSize) });
    if (pageToken) {
      params.set("pageToken", pageToken);
    }
    return request<FeedPage>(`/api/feed?${params.toString()}`, { userId });
  },
  createPost(userId: number, content: string) {
    return request<CreatedPost>("/api/posts", {
      method: "POST",
      userId,
      body: JSON.stringify({ content }),
    });
  },
  follow(userId: number, followeeId: number) {
    return request(`/api/follows`, {
      method: "POST",
      userId,
      body: JSON.stringify({ followeeId }),
    });
  },
  unfollow(userId: number, followeeId: number) {
    return request<void>(`/api/follows/${followeeId}`, { method: "DELETE", userId });
  },
  followers(userId: number, limit = 50) {
    return request<CursorPage<User>>(`/api/users/${userId}/followers?limit=${limit}`);
  },
  following(userId: number, limit = 50) {
    return request<CursorPage<User>>(`/api/users/${userId}/following?limit=${limit}`);
  },
  adminMetrics() {
    return request<AdminMetrics>("/api/admin/metrics");
  },
};
