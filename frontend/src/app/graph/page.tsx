"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useCurrentUser } from "@/lib/providers";
import { isCelebrity, type User } from "@/lib/types";

export default function GraphRoute() {
  const { user, users } = useCurrentUser();
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);

  const followingQuery = useQuery({
    queryKey: ["following", user?.id],
    enabled: Boolean(user),
    queryFn: () => api.following(user!.id),
  });
  const followersQuery = useQuery({
    queryKey: ["followers", user?.id],
    enabled: Boolean(user),
    queryFn: () => api.followers(user!.id),
  });

  const followingIds = new Set((followingQuery.data?.items ?? []).map((item) => item.id));

  const follow = useMutation({
    mutationFn: (followeeId: number) => api.follow(user!.id, followeeId),
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["following", user?.id] });
      void queryClient.invalidateQueries({ queryKey: ["users"] });
      void queryClient.invalidateQueries({ queryKey: ["feed", user?.id] });
    },
    onError: (cause) => {
      setError(cause instanceof ApiError ? cause.message : "Follow failed");
    },
  });
  const unfollow = useMutation({
    mutationFn: (followeeId: number) => api.unfollow(user!.id, followeeId),
    onSuccess: () => {
      setError(null);
      void queryClient.invalidateQueries({ queryKey: ["following", user?.id] });
      void queryClient.invalidateQueries({ queryKey: ["users"] });
      void queryClient.invalidateQueries({ queryKey: ["feed", user?.id] });
    },
    onError: (cause) => {
      setError(cause instanceof ApiError ? cause.message : "Unfollow failed");
    },
  });

  if (!user) {
    return <p className="text-sm text-zinc-500">Pick a user in the switcher to explore the graph.</p>;
  }

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-xl border border-black/10 p-4 dark:border-white/10">
        <h2 className="text-lg font-semibold">{user.displayName}</h2>
        <p className="text-sm text-zinc-500">
          @{user.username} · {user.followerCount} followers
          {isCelebrity(user) ? " · celebrity" : ""}
        </p>
      </section>
      {error ? <p className="text-sm text-red-600">{error}</p> : null}
      <section>
        <h3 className="mb-2 font-medium">All users</h3>
        <ul className="flex flex-col gap-2">
          {users
            .filter((candidate) => candidate.id !== user.id)
            .map((candidate) => (
              <UserRow
                key={candidate.id}
                user={candidate}
                following={followingIds.has(candidate.id)}
                busy={follow.isPending || unfollow.isPending}
                onFollow={() => follow.mutate(candidate.id)}
                onUnfollow={() => unfollow.mutate(candidate.id)}
              />
            ))}
        </ul>
      </section>
      <PeopleList title="Following" people={followingQuery.data?.items ?? []} />
      <PeopleList title="Followers" people={followersQuery.data?.items ?? []} />
    </div>
  );
}

function UserRow({
  user,
  following,
  busy,
  onFollow,
  onUnfollow,
}: {
  user: User;
  following: boolean;
  busy: boolean;
  onFollow: () => void;
  onUnfollow: () => void;
}) {
  return (
    <li className="flex items-center justify-between gap-3 rounded-lg border border-black/10 px-3 py-2 dark:border-white/10">
      <div>
        <p className="text-sm font-medium">{user.displayName}</p>
        <p className="text-xs text-zinc-500">
          @{user.username} · {user.followerCount} followers
          {isCelebrity(user) ? " · celebrity" : ""}
        </p>
      </div>
      <button
        type="button"
        disabled={busy}
        onClick={following ? onUnfollow : onFollow}
        className="rounded-md border border-black/15 px-3 py-1 text-sm dark:border-white/20"
      >
        {following ? "Unfollow" : "Follow"}
      </button>
    </li>
  );
}

function PeopleList({ title, people }: { title: string; people: User[] }) {
  return (
    <section>
      <h3 className="mb-2 font-medium">
        {title} ({people.length})
      </h3>
      {people.length === 0 ? (
        <p className="text-sm text-zinc-500">None yet.</p>
      ) : (
        <ul className="flex flex-col gap-1 text-sm">
          {people.map((person) => (
            <li key={person.id}>
              {person.displayName}{" "}
              <span className="text-zinc-500">@{person.username}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
