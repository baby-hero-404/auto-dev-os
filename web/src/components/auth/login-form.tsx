"use client";

import { FormEvent, useState } from "react";
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
      saveSession({ token: response.tokens.access_token, user: response.user });
    } catch (err) {
      setAuthError(err instanceof Error ? err.message : "Authentication failed");
    }
  }

  return (
    <main className="grid min-h-screen place-items-center px-4 py-10">
      <section className="w-full max-w-md rounded-lg border border-[var(--border)] bg-[var(--primary)] p-6 shadow-xl">
        <div className="mb-8 flex items-center gap-3">
          <img src="/logo.png" alt="Auto Code OS Logo" className="size-11 rounded-md object-contain" />
          <div>
            <h1 className="font-mono text-xl font-semibold">Auto Code OS</h1>
            <p className="text-sm text-[var(--muted)]">Secure AI SDLC control plane</p>
          </div>
        </div>

        <div className="mb-5 grid grid-cols-2 rounded-md border border-[var(--border)] p-1">
          <button
            className={`rounded px-3 py-2 text-sm transition ${mode === "login" ? "bg-[var(--accent)] text-slate-950" : "text-slate-300 hover:bg-slate-700"}`}
            onClick={() => setMode("login")}
            type="button"
          >
            Login
          </button>
          <button
            className={`rounded px-3 py-2 text-sm transition ${mode === "register" ? "bg-[var(--accent)] text-slate-950" : "text-slate-300 hover:bg-slate-700"}`}
            onClick={() => setMode("register")}
            type="button"
          >
            Register
          </button>
        </div>

        <form className="space-y-4" onSubmit={handleAuth}>
          <label className="block text-sm">
            <span className="mb-2 block text-slate-300">Email</span>
            <input
              name="email"
              type="email"
              required
              className="w-full rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-white"
            />
          </label>
          <label className="block text-sm">
            <span className="mb-2 block text-slate-300">Password</span>
            <input
              name="password"
              type="password"
              minLength={8}
              required
              className="w-full rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-white"
            />
          </label>
          {mode === "register" && (
            <label className="block text-sm">
              <span className="mb-2 block text-slate-300">Organization</span>
              <input
                name="org_name"
                className="w-full rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-white"
              />
            </label>
          )}
          {authError && (
            <p className="rounded-md border border-red-400/40 bg-red-950/40 px-3 py-2 text-sm text-red-100">
              {authError}
            </p>
          )}
          <button
            className="flex w-full items-center justify-center gap-2 rounded-md bg-[var(--accent)] px-4 py-2 font-semibold text-slate-950 transition hover:opacity-90"
            type="submit"
          >
            <Lock size={16} />
            Continue
          </button>
        </form>
      </section>
    </main>
  );
}
