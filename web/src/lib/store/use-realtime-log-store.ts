"use client";

import { create } from "zustand";

export type RealtimeLog = {
  id: string;
  streamId?: string;
  source: "sandbox" | "workflow" | "agent";
  level: string;
  message: string;
  createdAt: string;
  createdAtEpoch: number;
};

type RealtimeLogState = {
  logs: RealtimeLog[];
  appendLog: (log: RealtimeLog) => void;
  appendLogs: (logs: RealtimeLog[]) => void;
  clearLogs: (streamId?: string) => void;
};

const maxBufferedLogs = 500;

export const useRealtimeLogStore = create<RealtimeLogState>((set) => ({
  logs: [],
  appendLog: (log) =>
    set((state) => ({
      logs: appendUniqueLogs(state.logs, [normalizeLog(log)]),
    })),
  appendLogs: (logs) =>
    set((state) =>
      logs.length === 0
        ? state
        : { logs: appendUniqueLogs(state.logs, logs.map(normalizeLog)) },
    ),
  clearLogs: (streamId) =>
    set((state) => ({
      logs: streamId ? state.logs.filter((log) => log.streamId !== streamId) : [],
    })),
}));

function appendUniqueLogs(current: RealtimeLog[], incoming: RealtimeLog[]) {
  const seen = new Set(current.map((log) => log.id));
  let next = current;

  for (const log of incoming) {
    if (seen.has(log.id)) continue;
    if (next === current) next = [...current];
    seen.add(log.id);
    next.push(log);
  }

  return next.length > maxBufferedLogs ? next.slice(-maxBufferedLogs) : next;
}

function normalizeLog(log: RealtimeLog): RealtimeLog {
  return {
    ...log,
    createdAtEpoch: log.createdAtEpoch || Date.parse(log.createdAt),
  };
}
