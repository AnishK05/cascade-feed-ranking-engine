export type User = {
  id: number;
  username: string;
  displayName: string;
  isCelebrity?: boolean;
  celebrity?: boolean;
  followerCount: number;
  createdAt: string;
};

export type Author = {
  id: number;
  username: string;
  displayName: string;
  celebrity: boolean;
};

export type FeedItem = {
  postId: number;
  authorId: number;
  content: string;
  mediaUrl: string;
  createdAtUnixMs: number;
  rankScore: number;
  recencyScore: number;
  engagementScore: number;
  affinityScore: number;
  author: Author | null;
};

export type FeedPage = {
  items: FeedItem[];
  nextPageToken: string;
};

export type CreatedPost = {
  postId: number;
  authorId: number;
  createdAtUnixMs: number;
};

export type CursorPage<T> = {
  items: T[];
  nextCursor: string | null;
};

export type AdminMetrics = {
  requestsPerSecond: number;
  feedLatencyMs: { p50: number; p95: number; p99: number };
  cacheHitRatio: number;
  fanoutEventsPerSecond: number;
  fanoutLagMs: number;
  kafkaConsumerLag: number;
  available: boolean;
};

export function isCelebrity(user: Pick<User, "isCelebrity" | "celebrity">): boolean {
  return Boolean(user.isCelebrity ?? user.celebrity);
}
