import { request } from "./client";
import type { Attestation, AttestationVerifyResult } from "../types";

export function listByTask(taskID: string, token: string) {
  return request<Attestation[]>(`/tasks/${taskID}/attestations`, { token });
}

export function getByCommit(commit: string, token: string) {
  return request<AttestationVerifyResult>(`/attestations/${commit}`, { token });
}
