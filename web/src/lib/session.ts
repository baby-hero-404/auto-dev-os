"use client";

import { useSyncExternalStore } from "react";
import type { AuthResponse } from "./types";

export type Session = {
  token: string;
  user: AuthResponse["user"];
};

const sessionKey = "autocodeos.session";
const sessionEvent = "autocodeos-session";

function subscribe(callback: () => void) {
  window.addEventListener("storage", callback);
  window.addEventListener(sessionEvent, callback);
  return () => {
    window.removeEventListener("storage", callback);
    window.removeEventListener(sessionEvent, callback);
  };
}

let cachedSession: Session | null = null;
let lastRawValue: string | null = null;

function getSnapshot(): Session | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(sessionKey);
  if (raw !== lastRawValue) {
    lastRawValue = raw;
    cachedSession = raw ? (JSON.parse(raw) as Session) : null;
  }
  return cachedSession;
}

function getServerSnapshot(): Session | null {
  return null;
}

export function useSession() {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

export function saveSession(session: Session) {
  localStorage.setItem(sessionKey, JSON.stringify(session));
  window.dispatchEvent(new Event(sessionEvent));
}

export function clearSession() {
  localStorage.removeItem(sessionKey);
  window.dispatchEvent(new Event(sessionEvent));
}
