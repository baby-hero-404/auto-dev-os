"use client";

import { FormEvent, useState } from "react";
import Image from "next/image";
import { Lock } from "lucide-react";
import { api } from "@/lib/api";
import { saveSession } from "@/lib/session";

export function LoginForm() {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [authError, setAuthError] = useState("");

  async function handleAuth(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setAuthError("");
    const form = new FormData(event.currentTarget);
    const email = String(form.get("email") ?? "");
    const password = String(form.get("password") ?? "");
    const orgName = String(form.get("org_name") ?? "");
    try {
      const response =
        mode === "login"
          ? await api.login({ email, password })
          : await api.register({ email, password, org_name: orgName });
      saveSession({
        token: response.tokens.access_token,
        refresh_token: response.tokens.refresh_token,
        user: response.user,
      });
    } catch (err) {
      setAuthError(err instanceof Error ? err.message : "Authentication failed");
    }
  }

  return (
    <main className="relative grid min-h-screen place-items-center overflow-hidden bg-background px-4 py-12">
      {/* Background Decorative Mesh Gradients */}
      <div className="pointer-events-none absolute -left-48 -top-48 size-96 rounded-full bg-brand-primary/10 blur-[120px]" />
      <div className="pointer-events-none absolute -right-48 -bottom-48 size-96 rounded-full bg-accent/10 blur-[120px]" />

      <section className="relative z-10 w-full max-w-[420px] rounded-xl border border-stroke bg-card p-8 shadow-xl transition-all duration-200">
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="mb-4 overflow-hidden rounded-xl bg-brand-primary/10 p-2 shadow-inner">
            <Image
              src="/logo.png"
              alt="Auto Code OS Logo"
              width={48}
              height={48}
              className="rounded-lg object-contain"
            />
          </div>
          <h1 className="font-heading text-2xl font-bold tracking-tight text-foreground">Auto Code OS</h1>
          <p className="mt-1.5 text-sm text-content-muted">Secure AI SDLC control plane</p>
        </div>

        {/* Tab Toggle */}
        <div className="mb-6 grid grid-cols-2 rounded-lg bg-surface p-1 border border-stroke">
          <button
            type="button"
            onClick={() => setMode("login")}
            className={`rounded-md py-1.5 text-sm font-semibold transition-all duration-200 cursor-pointer ${
              mode === "login"
                ? "bg-card text-foreground shadow-sm border border-stroke/50"
                : "text-content-muted hover:text-foreground"
            }`}
          >
            Login
          </button>
          <button
            type="button"
            onClick={() => setMode("register")}
            className={`rounded-md py-1.5 text-sm font-semibold transition-all duration-200 cursor-pointer ${
              mode === "register"
                ? "bg-card text-foreground shadow-sm border border-stroke/50"
                : "text-content-muted hover:text-foreground"
            }`}
          >
            Register
          </button>
        </div>

        <form className="space-y-4" onSubmit={handleAuth}>
          <label className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Email</span>
            <input
              name="email"
              type="email"
              required
              className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              placeholder="you@example.com"
            />
          </label>

          <label className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Password</span>
            <input
              name="password"
              type="password"
              minLength={8}
              required
              className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              placeholder="••••••••"
            />
          </label>

          {mode === "register" && (
            <label className="flex flex-col gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Organization Name</span>
              <input
                name="org_name"
                required
                className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
                placeholder="Acme Corp"
              />
            </label>
          )}

          {authError && (
            <div className="rounded border border-red-500/20 bg-red-500/10 p-3 text-xs text-red-500 font-medium leading-relaxed">
              {authError}
            </div>
          )}

          <button
            type="submit"
            className="mt-6 flex w-full items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 font-semibold text-white shadow-lg shadow-brand-primary/10 hover:opacity-90 hover:shadow-brand-primary/20 active:scale-[0.98] transition-all duration-150 cursor-pointer"
          >
            <Lock size={15} />
            Continue
          </button>
        </form>
      </section>
    </main>
  );
}
