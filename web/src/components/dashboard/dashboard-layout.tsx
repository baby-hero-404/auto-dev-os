"use client";

import { useSession } from "@/lib/session";
import { HomeSidebar } from "@/components/dashboard/home/home-sidebar";
import { HomeHeader } from "@/components/dashboard/home/home-header";
import { LoginForm } from "@/components/auth/login-form";

export function DashboardLayout({ children }: { children: React.ReactNode }) {
  const session = useSession();

  if (!session) {
    return <LoginForm />;
  }

  return (
    <main className="flex min-h-screen">
      <HomeSidebar />
      <section className="flex min-w-0 flex-1 flex-col">
        <HomeHeader />
        <div className="flex-1 p-5">{children}</div>
      </section>
    </main>
  );
}
