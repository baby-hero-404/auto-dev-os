"use client";

import { useEffect } from "react";
import { useAuthStore, type Session } from "@/lib/store/use-auth-store";

export function useSession() {
  const session = useAuthStore((state) => state.session);
  const syncFromStorage = useAuthStore((state) => state.syncFromStorage);

  useEffect(() => {
    syncFromStorage();
    window.addEventListener("storage", syncFromStorage);
    return () => {
      window.removeEventListener("storage", syncFromStorage);
    };
  }, [syncFromStorage]);

  return session;
}

export function saveSession(session: Session) {
  useAuthStore.getState().setSession(session);
}

export function clearSession() {
  useAuthStore.getState().clearSession();
}

export type { Session };
