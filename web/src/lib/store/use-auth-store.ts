"use client";

import { create } from "zustand";
import type { AuthResponse } from "@/lib/types";

export type Session = {
  token: string;
  refresh_token: string;
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
  isHydrated: boolean;
  setSession: (session: Session) => void;
  clearSession: () => void;
  syncFromStorage: () => void;
};

export const useAuthStore = create<AuthState>((set) => ({
  session: null,
  isHydrated: false,
  setSession: (session) => {
    localStorage.setItem(sessionKey, JSON.stringify(session));
    set({ session, isHydrated: true });
  },
  clearSession: () => {
    localStorage.removeItem(sessionKey);
    set({ session: null, isHydrated: true });
  },
  syncFromStorage: () => {
    set({ session: readStoredSession(), isHydrated: true });
  },
}));

export { sessionKey };
