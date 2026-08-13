"use client";

import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { relativeTime } from "@/lib/format";
import { useCurrentUser } from "@/lib/providers";
import type { FeedItem, FeedPage } from "@/lib/types";

export default function FeedRoute() {
  const { user } = useCurrentUser();
  const queryClient = useQueryClient();
  const [content, setContent] = useState("");
  const [showDebug, setShowDebug] = useState(false);
  const [pendingPostId, setPendingPostId] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const sentinel = useRef<HTMLDivElement | null>(null);

  const feedQuery = useInfiniteQuery({
    queryKey: ["feed", user?.id],
    enabled: Boolean(user),
    queryFn: ({ pageParam }) => api.getFeed(user!.id, pageParam),
    initialPageParam: "",
    getNextPageParam: (lastPage) => lastPage.nextPageToken || undefined,
  });

  const { fetchNextPage, hasNextPage, isFetchingNextPage } = feedQuery;

  useEffect(() => {
    const node = sentinel.current;
    if (!node) {
      return;
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting) && hasNextPage && !isFetchingNextPage) {
        void fetchNextPage();
      }
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const items = useMemo(
    () => feedQuery.data?.pages.flatMap((page) => page.items) ?? [],
    [feedQuery.data],
  );
  const fanoutPending =
    pendingPostId != null &&
    !items.some((item) => item.postId === pendingPostId && item.authorId !== user?.id);

  const createPost = useMutation({
    mutationFn: (text: string) => api.createPost(user!.id, text),
    onSuccess: (created, text) => {
      setContent("");
      setPendingPostId(created.postId);
      queryClient.setQueryData<{ pages: FeedPage[]; pageParams: string[] }>(
        ["feed", user!.id],
        (current) => {
          const optimistic: FeedItem = {
            postId: created.postId,
            authorId: created.authorId,
            content: text,
            mediaUrl: "",
            createdAtUnixMs: created.createdAtUnixMs,
            rankScore: 0,
            recencyScore: 0,
            engagementScore: 0,
            affinityScore: 0,
            author: user
              ? {
                  id: user.id,
                  username: user.username,
                  displayName: user.displayName,
                  celebrity: Boolean(user.isCelebrity ?? user.celebrity),
                }
              : null,
          };
          if (!current) {
            return { pages: [{ items: [optimistic], nextPageToken: "" }], pageParams: [""] };
          }
          const [first, ...rest] = current.pages;
          return {
            ...current,
            pages: [{ ...first, items: [optimistic, ...first.items] }, ...rest],
          };
        },
      );
    },
    onError: (cause) => {
      setError(cause instanceof ApiError ? cause.message : "Could not create post");
    },
  });

  if (!user) {
    return (
      <p className="text-sm text-zinc-500">
        Create a user through Social Graph or seed the database, then refresh to pick an identity.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <form
        className="rounded-xl border border-black/10 p-4 dark:border-white/10"
        onSubmit={(event) => {
          event.preventDefault();
          setError(null);
          const text = content.trim();
          if (!text) {
            return;
          }
          createPost.mutate(text);
        }}
      >
        <label className="block text-sm font-medium">Compose</label>
        <textarea
          className="mt-2 w-full resize-y rounded-md border border-black/15 bg-transparent p-3 text-sm dark:border-white/20"
          rows={3}
          maxLength={5000}
          placeholder={`What's happening, ${user.displayName}?`}
          value={content}
          onChange={(event) => setContent(event.target.value)}
        />
        <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
          <p className="text-xs text-zinc-500">
            Your own post prepends immediately. Followers see it after Kafka fanout.
          </p>
          <button
            type="submit"
            disabled={createPost.isPending || content.trim().length === 0}
            className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm text-white disabled:opacity-40 dark:bg-zinc-100 dark:text-zinc-900"
          >
            {createPost.isPending ? "Posting…" : "Post"}
          </button>
        </div>
        {error ? <p className="mt-2 text-sm text-red-600">{error}</p> : null}
        {fanoutPending ? (
          <p className="mt-2 text-sm text-amber-700 dark:text-amber-400">
            Fanning out post #{pendingPostId}… switch to a follower and refresh to watch it land.
          </p>
        ) : null}
      </form>

      <label className="flex items-center gap-2 text-sm text-zinc-500">
        <input
          type="checkbox"
          checked={showDebug}
          onChange={(event) => setShowDebug(event.target.checked)}
        />
        Show rank breakdown
      </label>

      {feedQuery.isLoading ? <p className="text-sm text-zinc-500">Loading feed…</p> : null}
      {feedQuery.error instanceof Error ? (
        <p className="text-sm text-red-600">{feedQuery.error.message}</p>
      ) : null}

      <ol className="flex flex-col gap-3">
        {items.map((item) => (
          <li
            key={item.postId}
            className="rounded-xl border border-black/10 p-4 dark:border-white/10"
          >
            <div className="flex items-baseline justify-between gap-3">
              <p className="font-medium">
                {item.author?.displayName ?? `User ${item.authorId}`}
                <span className="ml-2 text-sm font-normal text-zinc-500">
                  @{item.author?.username ?? item.authorId}
                </span>
              </p>
              <time className="text-xs text-zinc-500">{relativeTime(item.createdAtUnixMs)}</time>
            </div>
            <p className="mt-2 whitespace-pre-wrap text-sm leading-6">{item.content}</p>
            {showDebug ? (
              <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1 text-xs text-zinc-500 sm:grid-cols-4">
                <div>
                  <dt>score</dt>
                  <dd>{item.rankScore.toFixed(3)}</dd>
                </div>
                <div>
                  <dt>recency</dt>
                  <dd>{item.recencyScore.toFixed(3)}</dd>
                </div>
                <div>
                  <dt>engagement</dt>
                  <dd>{item.engagementScore.toFixed(3)}</dd>
                </div>
                <div>
                  <dt>affinity</dt>
                  <dd>{item.affinityScore.toFixed(3)}</dd>
                </div>
              </dl>
            ) : null}
          </li>
        ))}
      </ol>
      <div ref={sentinel} className="h-8" />
      {feedQuery.isFetchingNextPage ? (
        <p className="text-center text-sm text-zinc-500">Loading more…</p>
      ) : null}
      {!feedQuery.isLoading && items.length === 0 ? (
        <p className="text-sm text-zinc-500">
          Empty timeline. Follow someone on the Graph page, then have them post.
        </p>
      ) : null}
    </div>
  );
}
