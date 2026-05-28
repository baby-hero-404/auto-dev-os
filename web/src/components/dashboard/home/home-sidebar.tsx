"use client";

import { FolderKanban, Wrench, Building, Settings, Zap, BookOpen, Bot, BarChart3 } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

const navItems = [
  { label: "Projects", href: "/", icon: FolderKanban },
  { label: "Agents", href: "/agents", icon: Bot },
  { label: "Skills", href: "/skills", icon: Zap },
  { label: "Gateway", href: "/gateway", icon: BarChart3 },
  { label: "Rules", href: "/rules", icon: Wrench },
  { label: "Knowledge", href: "/knowledge", icon: BookOpen },
  { label: "Organization", href: "/organization", icon: Building },
  { label: "Settings", href: "/settings", icon: Settings },
];

export function HomeSidebar() {
  const pathname = usePathname();

  return (
    <aside className="hidden w-72 shrink-0 border-r border-[var(--border)] bg-slate-950/50 p-5 md:block">
      <div className="mb-8 flex items-center gap-3">
        <img src="/logo.png" alt="Auto Code OS Logo" className="size-10 rounded-md object-contain" />
        <div>
          <div className="font-mono font-semibold">Auto Code OS</div>
          <div className="text-xs text-[var(--muted)]">AI-Native SDLC</div>
        </div>
      </div>
      <nav className="space-y-1 text-sm">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = pathname === item.href;
          return (
            <Link
              key={item.label}
              href={item.href}
              className={`flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-left transition ${
                isActive
                  ? "bg-[var(--accent)]/10 text-[var(--accent)]"
                  : "text-slate-300 hover:bg-[var(--primary)]"
              }`}
            >
              <Icon size={17} />
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
