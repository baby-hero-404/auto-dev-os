import { request } from "./client";
import type { GovernancePreset } from "../types";

export function listPresets(token: string) {
  return request<GovernancePreset[]>("/governance/presets", { token });
}
