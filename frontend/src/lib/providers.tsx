"use client";

import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { createContext, useCallback, useContext, useMemo, useState, useSyncExternalStore } from "react";
import { api } from "./api";
import type { User } from "./types";

const STORAGE_KEY = "cascade.userId";

const listeners = new Set<() => void>();

function emitUserChange() {
  for (const listener of listeners) {
    listener();
  }
}

function subscribe(onStoreChange: () => void) {
  listeners.add(onStoreChange);
  window.addEventListener("storage", onStoreChange);
  return () => {
    listeners.delete(onStoreChange);
    window.removeEventListener("storage", onStoreChange);
  };
}

function readStoredUserId() {
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (!stored) {
    return null;
  }
  const parsed = Number(stored);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

type UserState = {
  users: User[];
  user: User | null;
  userId: number | null;
  setUserId: (id: number) => void;
};

const UserContext = createContext<UserState | null>(null);

function UserProvider({ children }: { children: React.ReactNode }) {
  const storedId = useSyncExternalStore(subscribe, readStoredUserId, () => null);
  const usersQuery = useQuery({
    queryKey: ["users"],
    queryFn: () => api.listUsers(100),
    staleTime: 15_000,
  });
  const users = useMemo(() => usersQuery.data ?? [], [usersQuery.data]);
  const userId =
    storedId && users.some((candidate) => candidate.id === storedId)
      ? storedId
      : (users[0]?.id ?? null);
  const user = users.find((candidate) => candidate.id === userId) ?? null;

  const setUserId = useCallback((id: number) => {
    window.localStorage.setItem(STORAGE_KEY, String(id));
    emitUserChange();
  }, []);

  const value = useMemo(
    () => ({ users, user, userId, setUserId }),
    [users, user, userId, setUserId],
  );

  return <UserContext.Provider value={value}>{children}</UserContext.Provider>;
}

export function useCurrentUser() {
  const context = useContext(UserContext);
  if (!context) {
    throw new Error("useCurrentUser must be used within Providers");
  }
  return context;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { refetchOnWindowFocus: false, retry: 1 } },
      }),
  );
  return (
    <QueryClientProvider client={client}>
      <UserProvider>{children}</UserProvider>
    </QueryClientProvider>
  );
}
