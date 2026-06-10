"use client";

import { Bell, FolderKanban, LogOut, Menu, Search } from "lucide-react";
import { clearSession, useSession } from "@/lib/session";
import { useProjectStore } from "@/lib/store/use-project-store";

export function HomeHeader({ onMenuClick }: { onMenuClick?: () => void }) {
  const session = useSession();
  const activeProjectId = useProjectStore((state) => state.activeProjectId);

  return (
    <header className="flex shrink-0 items-center justify-between border-b border-stroke bg-page px-4 py-3 sm:px-5 sm:py-4">
      <button
        className="mr-3 rounded-md border border-stroke p-2 transition hover:bg-panel md:hidden"
        onClick={onMenuClick}
        title="Open navigation"
        type="button"
      >
        <Menu size={17} />
      </button>
      <div className="flex min-w-0 flex-1 items-center gap-3 rounded-md border border-stroke bg-page px-3 py-2 text-content-muted md:max-w-xl">
        <Search size={17} />
        <span className="truncate text-sm">Search projects, tasks, agents…</span>
      </div>
      <div className="ml-4 flex items-center gap-2">
        {activeProjectId && (
          <div className="hidden items-center gap-2 rounded-md border border-stroke bg-panel px-2.5 py-2 font-mono text-xs text-content-muted xl:flex">
            <FolderKanban size={14} className="text-brand-primary" />
            <span>{activeProjectId.slice(0, 8)}</span>
          </div>
        )}
        {session && (
          <span className="hidden text-sm text-content-muted lg:block">
            {session.user.email}
          </span>
        )}
        <button
          className="rounded-md border border-stroke p-2 transition hover:bg-panel"
          title="Notifications"
        >
          <Bell size={17} />
        </button>
        <button
          className="rounded-md border border-stroke p-2 transition hover:bg-panel"
          onClick={clearSession}
          title="Logout"
        >
          <LogOut size={17} />
        </button>
      </div>
    </header>
  );
}
