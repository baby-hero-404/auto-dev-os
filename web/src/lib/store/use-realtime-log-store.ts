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
  droppedCount: number;
  appendLog: (log: RealtimeLog) => void;
  appendLogs: (logs: RealtimeLog[]) => void;
  clearLogs: (streamId?: string) => void;
  replaceLogs: (streamId: string, logs: RealtimeLog[]) => void;
};

// Virtuoso virtualizes rendering, so the real cost of a larger buffer is memory,
// not render time — 5000 lines covers effectively all real CLI/API-native transcripts
// while `droppedCount` still tells the user if anything was ever trimmed.
const maxBufferedLogs = 5000;

export const useRealtimeLogStore = create<RealtimeLogState>((set) => ({
  logs: [],
  droppedCount: 0,
  appendLog: (log) =>
    set((state) => appendUniqueLogs(state, [normalizeLog(log)])),
  appendLogs: (logs) =>
    set((state) => (logs.length === 0 ? state : appendUniqueLogs(state, logs.map(normalizeLog)))),
  clearLogs: (streamId) =>
    set((state) => ({
      logs: streamId ? state.logs.filter((log) => log.streamId !== streamId) : [],
      droppedCount: 0,
    })),
  replaceLogs: (streamId, logs) =>
    set((state) => ({
      logs: [...state.logs.filter((log) => log.streamId !== streamId), ...logs.map(normalizeLog)],
      droppedCount: 0,
    })),
}));

function appendUniqueLogs(state: RealtimeLogState, incoming: RealtimeLog[]) {
  const seen = new Set(state.logs.map((log) => log.id));
  let next = state.logs;

  for (const log of incoming) {
    if (seen.has(log.id)) continue;
    if (next === state.logs) next = [...state.logs];
    seen.add(log.id);
    next.push(log);
  }

  if (next.length <= maxBufferedLogs) return { logs: next, droppedCount: state.droppedCount };

  const overflow = next.length - maxBufferedLogs;
  return { logs: next.slice(-maxBufferedLogs), droppedCount: state.droppedCount + overflow };
}

function normalizeLog(log: RealtimeLog): RealtimeLog {
  return {
    ...log,
    createdAtEpoch: log.createdAtEpoch || Date.parse(log.createdAt),
  };
}
