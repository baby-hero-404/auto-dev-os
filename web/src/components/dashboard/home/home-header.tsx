"use client";

import { Bell, LogOut, Search } from "lucide-react";
import { clearSession, useSession } from "@/lib/session";

export function HomeHeader() {
  const session = useSession();

  return (
    <header className="flex items-center justify-between border-b border-[var(--border)] bg-[var(--background)] px-5 py-4">
      <div className="flex min-w-0 flex-1 items-center gap-3 rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-slate-400 md:max-w-xl">
        <Search size={17} />
        <span className="truncate text-sm">Search projects, tasks, agents…</span>
      </div>
      <div className="ml-4 flex items-center gap-2">
        {session && (
          <span className="hidden text-sm text-[var(--muted)] lg:block">
            {session.user.email}
          </span>
        )}
        <button
          className="rounded-md border border-[var(--border)] p-2 transition hover:bg-[var(--primary)]"
          title="Notifications"
        >
          <Bell size={17} />
        </button>
        <button
          className="rounded-md border border-[var(--border)] p-2 transition hover:bg-[var(--primary)]"
          onClick={clearSession}
          title="Logout"
        >
          <LogOut size={17} />
        </button>
      </div>
    </header>
  );
}
