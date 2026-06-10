"use client";

import { create } from "zustand";

type ProjectState = {
  activeProjectId: string | null;
  setActiveProjectId: (projectID: string | null) => void;
};

export const useProjectStore = create<ProjectState>((set) => ({
  activeProjectId: null,
  setActiveProjectId: (projectID) => set({ activeProjectId: projectID }),
}));
