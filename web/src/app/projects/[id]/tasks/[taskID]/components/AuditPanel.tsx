"use client";

import { useState, useEffect } from "react";
import useSWR from "swr";
import { ShieldCheck, ShieldAlert, Key, Calendar, Code, CheckCircle, FileCode, X, Loader2 } from "lucide-react";
import { useTaskDetail } from "./TaskDetailContext";
import { attestations as attestationsApi } from "@/lib/api";
import type { Attestation, AttestationVerifyResult } from "@/lib/types";

function AttestationRow({ attestation, token }: { attestation: Attestation; token: string }) {
  const [verifyResult, setVerifyResult] = useState<AttestationVerifyResult | null>(null);
  const [isVerifying, setIsVerifying] = useState(false);
  const [showModal, setShowModal] = useState(false);

  useEffect(() => {
    let isMounted = true;
    if (attestation.commit_hash && token) {
      setIsVerifying(true);
      attestationsApi
        .getByCommit(attestation.commit_hash, token)
        .then((res) => {
          if (isMounted) setVerifyResult(res);
        })
        .catch(() => {
          if (isMounted) setVerifyResult(null);
        })
        .finally(() => {
          if (isMounted) setIsVerifying(false);
        });
    }
    return () => {
      isMounted = false;
    };
  }, [attestation.commit_hash, token]);

  const shortCommit = attestation.commit_hash ? attestation.commit_hash.slice(0, 7) : "—";
  const formattedTime = new Date(attestation.created_at).toLocaleString();

  const codedByStr = attestation.coded_by
    ? `${attestation.coded_by.provider}/${attestation.coded_by.model}${
        attestation.coded_by.engine ? ` (${attestation.coded_by.engine})` : ""
      }`
    : "—";

  const reviewedByStr = attestation.reviewed_by
    ? `${attestation.reviewed_by.provider}/${attestation.reviewed_by.model}`
    : "—";

  return (
    <>
      <div className="p-3.5 text-xs flex flex-col gap-2 hover:bg-slate-500/5 transition-colors duration-150">
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <span className="font-mono font-bold text-foreground bg-slate-500/10 px-2 py-0.5 rounded text-[11px]">
              {shortCommit}
            </span>
            <span className="text-[10px] text-content-muted flex items-center gap-1 font-mono">
              <Key size={11} /> {attestation.key_id}
            </span>
          </div>

          {isVerifying ? (
            <span className="text-[9px] font-bold uppercase px-2 py-0.5 rounded-full bg-slate-500/10 text-content-muted flex items-center gap-1">
              <Loader2 size={10} className="animate-spin" /> Verifying
            </span>
          ) : verifyResult ? (
            verifyResult.verified ? (
              <span className="text-[9px] font-bold uppercase px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 flex items-center gap-1">
                <ShieldCheck size={11} /> Verified ✓
              </span>
            ) : (
              <span className="text-[9px] font-bold uppercase px-2 py-0.5 rounded-full bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20 flex items-center gap-1">
                <ShieldAlert size={11} /> Tampered ✗
              </span>
            )
          ) : (
            <span className="text-[9px] font-bold uppercase px-2 py-0.5 rounded-full bg-slate-500/10 text-content-muted">
              Unverified
            </span>
          )}
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px] font-mono text-content-muted pt-1">
          <div className="flex items-center gap-1.5 truncate">
            <Code size={12} className="text-brand-primary shrink-0" />
            <span>Coded by: <strong className="text-foreground">{codedByStr}</strong></span>
          </div>
          <div className="flex items-center gap-1.5 truncate">
            <CheckCircle size={12} className="text-emerald-500 shrink-0" />
            <span>Reviewed by: <strong className="text-foreground">{reviewedByStr}</strong></span>
          </div>
        </div>

        <div className="flex items-center justify-between text-[10px] text-content-muted pt-1 border-t border-stroke/10 mt-1">
          <div className="flex items-center gap-1">
            <Calendar size={11} />
            <span>{formattedTime}</span>
          </div>
          {verifyResult?.envelope !== undefined && verifyResult?.envelope !== null && (
            <button
              type="button"
              onClick={() => setShowModal(true)}
              className="inline-flex items-center gap-1 text-brand-primary hover:underline font-semibold cursor-pointer"
            >
              <FileCode size={11} /> View envelope
            </button>
          )}
        </div>
      </div>

      {/* Raw Envelope Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-card border border-stroke/20 rounded-2xl max-w-2xl w-full max-h-[85vh] flex flex-col shadow-2xl overflow-hidden">
            <div className="p-4 border-b border-stroke/10 flex items-center justify-between bg-slate-500/5">
              <div className="flex items-center gap-2">
                <FileCode className="text-brand-primary" size={18} />
                <h3 className="font-heading text-sm font-bold text-foreground">
                  DSSE Envelope — {shortCommit}
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setShowModal(false)}
                className="p-1 rounded-lg hover:bg-slate-500/10 text-content-muted hover:text-foreground transition cursor-pointer"
              >
                <X size={16} />
              </button>
            </div>
            <div className="p-4 overflow-y-auto font-mono text-xs bg-slate-950 text-slate-200 custom-scrollbar">
              <pre className="whitespace-pre-wrap break-all">
                {JSON.stringify(verifyResult?.envelope, null, 2)}
              </pre>
            </div>
            <div className="p-3 border-t border-stroke/10 flex justify-end bg-slate-500/5">
              <button
                type="button"
                onClick={() => setShowModal(false)}
                className="px-4 py-1.5 rounded-lg text-xs font-semibold bg-secondary hover:bg-stroke text-foreground transition cursor-pointer"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

export function AuditPanel() {
  const { taskID, token } = useTaskDetail();

  const { data: attestations, isLoading } = useSWR(
    taskID && token ? [`/tasks/${taskID}/attestations`, token] : null,
    () => attestationsApi.listByTask(taskID, token)
  );

  if (isLoading) {
    return (
      <div className="p-4 text-xs font-mono text-content-muted flex items-center gap-2">
        <Loader2 size={14} className="animate-spin text-brand-primary" /> Loading attestations...
      </div>
    );
  }

  if (!attestations || attestations.length === 0) {
    return (
      <div className="p-4 text-xs text-content-muted italic">
        No attestation records found for this task.
      </div>
    );
  }

  return (
    <div className="border border-stroke/10 rounded-2xl overflow-hidden bg-slate-500/[0.02] divide-y divide-stroke/10 shadow-sm">
      {attestations.map((attestation) => (
        <AttestationRow key={attestation.id} attestation={attestation} token={token} />
      ))}
    </div>
  );
}
