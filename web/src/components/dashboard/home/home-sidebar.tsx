"use client";

import { FolderKanban, Wrench, Building, Settings, Zap, BookOpen, Bot, BarChart3, TrendingUp, ShieldCheck, Cpu, KeyRound } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect } from "react";
import { useProjectStore } from "@/lib/store/use-project-store";

const navItems = [
  { label: "Projects", href: "/", icon: FolderKanban },
  { label: "Agents", href: "/agents", icon: Bot },
  { label: "Analytics", href: "/analytics", icon: TrendingUp },
  { label: "Skills", href: "/skills", icon: Zap },
  { label: "Gateway", href: "/gateway", icon: BarChart3 },
  { label: "Virtual Keys", href: "/settings/virtual-keys", icon: KeyRound },
  { label: "Audit Log", href: "/audit", icon: ShieldCheck },
  { label: "Rules", href: "/rules", icon: Wrench },
  { label: "Knowledge", href: "/knowledge", icon: BookOpen },
  { label: "Organization", href: "/organization", icon: Building },
  { label: "AI Providers", href: "/ai-providers", icon: Cpu },
  { label: "Settings", href: "/settings", icon: Settings },
];

type HomeSidebarProps = {
  variant?: "desktop" | "mobile";
  onNavigate?: () => void;
};

export function HomeSidebar({ variant = "desktop", onNavigate }: HomeSidebarProps = {}) {
  const pathname = usePathname();
  const setActiveProjectId = useProjectStore((state) => state.setActiveProjectId);
  const sidebarClassName =
    variant === "desktop"
      ? "hidden h-screen w-64 shrink-0 flex-col border-r border-stroke bg-surface md:flex"
      : "flex h-full w-72 max-w-[85vw] shrink-0 flex-col border-r border-stroke bg-surface";

  useEffect(() => {
    const match = pathname.match(/^\/projects\/([^/]+)/);
    setActiveProjectId(match?.[1] ?? null);
  }, [pathname, setActiveProjectId]);

  return (
    <aside className={sidebarClassName}>
      {/* Brand */}
      <div className="flex items-center gap-3 border-b border-stroke px-5 py-5">
        <Image src="/logo.png" alt="Auto Code OS Logo" width={36} height={36} className="rounded-lg object-contain" />
        <div>
          <div className="font-mono text-sm font-semibold tracking-tight">Auto Code OS</div>
          <div className="text-[11px] text-content-muted">AI-Native SDLC</div>
        </div>
      </div>

      {/* Nav */}
      <nav className="flex-1 space-y-0.5 overflow-y-auto p-3 text-sm">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive =
            item.href === "/"
              ? pathname === "/" || pathname.startsWith("/projects")
              : pathname === item.href || pathname.startsWith(`${item.href}/`);
          return (
            <Link
              key={item.label}
              href={item.href}
              onClick={onNavigate}
              className={`flex w-full items-center gap-3 rounded-lg px-3 py-2.5 font-medium transition-all duration-150 ${
                isActive
                  ? "bg-brand-primary-muted text-brand-primary shadow-[inset_0_0_0_1px_rgba(34,197,94,0.2)]"
                  : "text-content-muted hover:bg-panel hover:text-content"
              }`}
            >
              <Icon size={16} className="shrink-0" />
              <span>{item.label}</span>
            </Link>
          );
        })}
      </nav>

      {/* Version footer */}
      <div className="border-t border-stroke px-5 py-3">
        <span className="font-mono text-[10px] text-content-muted">v0.1.0-alpha</span>
      </div>
    </aside>
  );
}
