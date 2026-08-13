"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useCurrentUser } from "@/lib/providers";

const LINKS = [
  { href: "/feed", label: "Feed" },
  { href: "/graph", label: "Graph" },
  { href: "/admin", label: "Admin" },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { users, user, setUserId } = useCurrentUser();

  return (
    <div className="mx-auto flex min-h-full w-full max-w-3xl flex-col px-4 pb-16">
      <header className="sticky top-0 z-10 -mx-4 mb-6 border-b border-black/10 bg-[var(--background)]/90 px-4 py-3 backdrop-blur dark:border-white/10">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs uppercase tracking-[0.2em] text-zinc-500">Cascade</p>
            <h1 className="text-lg font-semibold">Feed ranking demo</h1>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <span className="text-zinc-500">Acting as</span>
            <select
              className="rounded-md border border-black/15 bg-transparent px-2 py-1 dark:border-white/20"
              value={user?.id ?? ""}
              onChange={(event) => setUserId(Number(event.target.value))}
              disabled={users.length === 0}
            >
              {users.length === 0 ? (
                <option value="">No users yet</option>
              ) : (
                users.map((candidate) => (
                  <option key={candidate.id} value={candidate.id}>
                    {candidate.displayName} (@{candidate.username})
                  </option>
                ))
              )}
            </select>
          </label>
        </div>
        <nav className="mt-3 flex gap-4 text-sm">
          {LINKS.map((link) => {
            const active = pathname === link.href;
            return (
              <Link
                key={link.href}
                href={link.href}
                className={
                  active
                    ? "font-semibold text-zinc-900 dark:text-zinc-100"
                    : "text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-200"
                }
              >
                {link.label}
              </Link>
            );
          })}
        </nav>
      </header>
      {children}
    </div>
  );
}
