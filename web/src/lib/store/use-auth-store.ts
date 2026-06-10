"use client";

import { create } from "zustand";
import type { AuthResponse } from "@/lib/types";

export type Session = {
  token: string;
  user: AuthResponse["user"];
};

const sessionKey = "autocodeos.session";

function readStoredSession(): Session | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(sessionKey);
  return raw ? (JSON.parse(raw) as Session) : null;
}

type AuthState = {
  session: Session | null;
  setSession: (session: Session) => void;
  clearSession: () => void;
  syncFromStorage: () => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  session: null,
  setSession: (session) => {
    localStorage.setItem(sessionKey, JSON.stringify(session));
    set({ session });
  },
  clearSession: () => {
    localStorage.removeItem(sessionKey);
    set({ session: null });
  },
  syncFromStorage: () => {
    set({ session: readStoredSession() });
  },
}));

export { sessionKey };
