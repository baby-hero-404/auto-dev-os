"use client";

import {
  FolderKanban,
  Wrench,
  Building,
  Zap,
  BookOpen,
  Bot,
  TrendingUp,
  ShieldCheck,
  Cpu,
  GitBranch,
  BarChart3,
} from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect } from "react";
import { useProjectStore } from "@/lib/store/use-project-store";

type NavItem = { label: string; href: string; icon: typeof FolderKanban };
type NavSection = { title: string; items: NavItem[] };

const navSections: NavSection[] = [
  {
    title: "Workspace",
    items: [
      { label: "Projects", href: "/", icon: FolderKanban },
      { label: "Agents", href: "/agents", icon: Bot },
      { label: "Skills", href: "/skills", icon: Zap },
      { label: "Rules", href: "/rules", icon: Wrench },
    ],
  },
  {
    title: "Infrastructure",
    items: [
      { label: "AI Providers", href: "/ai-providers", icon: Cpu },
      { label: "Gateway", href: "/gateway", icon: BarChart3 },
      { label: "Git Accounts", href: "/git-accounts", icon: GitBranch },
    ],
  },
  {
    title: "Intelligence",
    items: [
      { label: "Analytics", href: "/analytics", icon: TrendingUp },
      { label: "Audit Log", href: "/audit", icon: ShieldCheck },
      { label: "Knowledge", href: "/knowledge", icon: BookOpen },
    ],
  },
  {
    title: "Admin",
    items: [
      { label: "Organization", href: "/organization", icon: Building },
    ],
  },
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
      ? "hidden h-screen w-60 shrink-0 flex-col border-r border-stroke bg-card md:flex"
      : "flex h-full w-72 max-w-[85vw] shrink-0 flex-col border-r border-stroke bg-card";

  useEffect(() => {
    const match = pathname.match(/^\/projects\/([^/]+)/);
    setActiveProjectId(match?.[1] ?? null);
  }, [pathname, setActiveProjectId]);

  function isActive(href: string) {
    if (href === "/") return pathname === "/" || pathname.startsWith("/projects");
    return pathname === href || pathname.startsWith(`${href}/`);
  }

  return (
    <aside className={sidebarClassName}>
      {/* Brand */}
      <div className="flex items-center gap-3 border-b border-stroke px-5 py-4">
        <Image src="/logo.png" alt="Auto Code OS Logo" width={32} height={32} className="rounded-lg object-contain" />
        <div>
          <div className="text-sm font-semibold tracking-tight text-foreground">Auto Code OS</div>
          <div className="text-[11px] text-content-muted">AI-Native SDLC</div>
        </div>
      </div>

      {/* Nav sections */}
      <nav className="flex-1 overflow-y-auto px-2.5 py-3 text-[13px]">
        {navSections.map((section, idx) => (
          <div key={section.title} className={idx > 0 ? "mt-5" : ""}>
            <div className="mb-1.5 px-3 font-mono text-[10px] font-bold uppercase tracking-widest text-content-muted/60">
              {section.title}
            </div>
            <div className="space-y-0.5">
              {section.items.map((item) => {
                const Icon = item.icon;
                const active = isActive(item.href);
                return (
                  <Link
                    key={item.label}
                    href={item.href}
                    onClick={onNavigate}
                    className={`group flex w-full items-center gap-2.5 rounded-lg px-3 py-2 font-medium transition-all duration-150 ${active
                      ? "bg-brand-primary/10 text-brand-primary shadow-[inset_3px_0_0_0_var(--accent)]"
                      : "text-content-muted hover:bg-surface hover:text-foreground"
                      }`}
                  >
                    <Icon size={16} className={`shrink-0 transition ${active ? "" : "group-hover:text-brand-primary"}`} />
                    <span>{item.label}</span>
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      {/* Version footer */}
      <div className="border-t border-stroke px-5 py-3">
        <span className="font-mono text-[10px] text-content-muted">v0.1.0-alpha</span>
      </div>
    </aside>
  );
}
