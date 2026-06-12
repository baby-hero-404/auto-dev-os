"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { useSession } from "@/lib/session";
import { HomeSidebar } from "@/components/dashboard/home/home-sidebar";
import { HomeHeader } from "@/components/dashboard/home/home-header";
import { LoginForm } from "@/components/auth/login-form";

export function DashboardLayout({ children }: { children: React.ReactNode }) {
  const session = useSession();
  const [isNavOpen, setIsNavOpen] = useState(false);

  if (!session) {
    return <LoginForm />;
  }

  return (
    <main className="flex h-screen overflow-hidden bg-background">
      <HomeSidebar />
      <section className="flex min-h-0 min-w-0 flex-1 flex-col">
        <HomeHeader onMenuClick={() => setIsNavOpen(true)} />
        <div className="min-h-0 flex-1 overflow-y-auto bg-surface/50 px-4 py-5 sm:px-5 lg:px-6">
          <div className="mx-auto w-full max-w-[1600px]">{children}</div>
        </div>
      </section>

      {isNavOpen && (
        <div className="fixed inset-0 z-40 md:hidden" role="dialog" aria-modal="true">
          <button
            className="absolute inset-0 h-full w-full bg-foreground/20 backdrop-blur-sm"
            onClick={() => setIsNavOpen(false)}
            aria-label="Close navigation"
            type="button"
          />
          <div className="relative z-10 h-full">
            <button
              className="absolute right-3 top-3 z-20 rounded-lg border border-stroke bg-card p-2 transition hover:bg-surface"
              onClick={() => setIsNavOpen(false)}
              title="Close navigation"
              type="button"
            >
              <X size={17} />
            </button>
            <HomeSidebar variant="mobile" onNavigate={() => setIsNavOpen(false)} />
          </div>
        </div>
      )}
    </main>
  );
}
