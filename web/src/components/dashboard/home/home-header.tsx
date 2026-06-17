"use client";

import { Bell, FolderKanban, LogOut, Menu, Moon, Search, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { clearSession, useSession } from "@/lib/session";
import { useProjectStore } from "@/lib/store/use-project-store";

export function HomeHeader({ onMenuClick }: { onMenuClick?: () => void }) {
  const session = useSession();
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const { theme, setTheme } = useTheme();

  return (
    <header className="flex shrink-0 items-center justify-between border-b border-stroke bg-card px-4 py-3 sm:px-5">
      <button
        className="mr-3 rounded-lg border border-stroke p-2 transition hover:bg-surface md:hidden"
        onClick={onMenuClick}
        title="Open navigation"
        type="button"
      >
        <Menu size={17} />
      </button>

      {/* Search bar */}
      <div className="flex min-w-0 flex-1 items-center gap-2.5 rounded-lg border border-stroke bg-background px-3 py-2 text-content-muted md:max-w-md">
        <Search size={16} />
        <span className="truncate text-sm">Search projects, tasks, agents…</span>
      </div>

      <div className="ml-4 flex items-center gap-2">
        {activeProjectId && (
          <div className="hidden items-center gap-2 rounded-lg border border-stroke bg-surface px-2.5 py-1.5 font-mono text-xs text-content-muted xl:flex">
            <FolderKanban size={14} className="text-brand-primary" />
            <span>{activeProjectId.slice(0, 8)}</span>
          </div>
        )}
        {session && (
          <span className="hidden text-sm text-content-muted lg:block">
            {session.user.email}
          </span>
        )}

        {/* Theme toggle */}
        <button
          className="rounded-lg border border-stroke p-2 transition hover:bg-surface"
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          title="Toggle theme"
          type="button"
        >
          {theme === "dark" ? <Sun size={16} /> : <Moon size={16} />}
        </button>

        <button
          className="rounded-lg border border-stroke p-2 transition hover:bg-surface"
          title="Notifications"
          type="button"
        >
          <Bell size={16} />
        </button>
        <button
          className="rounded-lg border border-stroke p-2 transition hover:bg-surface"
          onClick={clearSession}
          title="Logout"
          type="button"
        >
          <LogOut size={16} />
        </button>
      </div>
    </header>
  );
}
