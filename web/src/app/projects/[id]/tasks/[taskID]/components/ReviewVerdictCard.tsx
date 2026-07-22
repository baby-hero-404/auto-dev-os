"use client";

import { useState } from "react";
import type { ReviewVerdict, SpecViolation, ReviewFinding } from "@/lib/types";
import { CheckCircle2, XCircle, ChevronDown, ChevronUp, AlertCircle } from "lucide-react";

interface ReviewVerdictCardProps {
  verdict: ReviewVerdict;
}

export function ReviewVerdictCard({ verdict }: ReviewVerdictCardProps) {
  const [showSpecDetails, setShowSpecDetails] = useState(false);
  const [showQualityDetails, setShowQualityDetails] = useState(false);

  const specPass = verdict.spec_compliance?.verdict?.toLowerCase() === "pass";
  const qualityPass = verdict.code_quality?.verdict?.toLowerCase() === "pass";

  const violations: SpecViolation[] = verdict.spec_compliance?.violations || [];
  const findings: ReviewFinding[] = verdict.code_quality?.findings || [];

  return (
    <div className="mt-2 p-3 rounded-xl border border-stroke/10 bg-slate-500/[0.03] space-y-2.5">
      <div className="text-[10px] font-bold uppercase tracking-wider text-content-muted">
        Structured Review Verdict
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
        {/* Spec Compliance Badge */}
        <div className="flex flex-col gap-1.5 p-2.5 rounded-lg border border-stroke/10 bg-background/50">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-foreground">Spec Compliance</span>
            <span
              className={`inline-flex items-center gap-1 text-[10px] font-bold uppercase px-2 py-0.5 rounded-full border ${
                specPass
                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
                  : "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20"
              }`}
            >
              {specPass ? (
                <>
                  <CheckCircle2 size={12} /> PASS
                </>
              ) : (
                <>
                  <XCircle size={12} /> FAIL
                </>
              )}
            </span>
          </div>

          {violations.length > 0 && (
            <button
              onClick={() => setShowSpecDetails(!showSpecDetails)}
              className="mt-1 flex items-center justify-between text-[10px] font-medium text-amber-600 dark:text-amber-400 hover:underline cursor-pointer"
            >
              <span>{violations.length} Spec Violation(s)</span>
              {showSpecDetails ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
            </button>
          )}

          {showSpecDetails && violations.length > 0 && (
            <div className="mt-2 space-y-2 text-[11px] border-t border-stroke/10 pt-2">
              {violations.map((v, i) => (
                <div key={i} className="p-2 rounded bg-rose-500/5 border border-rose-500/10 text-rose-700 dark:text-rose-300">
                  <div className="font-semibold">{v.requirement}</div>
                  <div className="text-[10px] opacity-90 mt-0.5">{v.explanation}</div>
                  {v.file && (
                    <div className="text-[9px] font-mono mt-1 opacity-75">
                      {v.file}:{v.line ?? 1}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Code Quality Badge */}
        <div className="flex flex-col gap-1.5 p-2.5 rounded-lg border border-stroke/10 bg-background/50">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-foreground">Code Quality</span>
            <span
              className={`inline-flex items-center gap-1 text-[10px] font-bold uppercase px-2 py-0.5 rounded-full border ${
                qualityPass
                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
                  : "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20"
              }`}
            >
              {qualityPass ? (
                <>
                  <CheckCircle2 size={12} /> PASS
                </>
              ) : (
                <>
                  <AlertCircle size={12} /> FAIL
                </>
              )}
            </span>
          </div>

          {findings.length > 0 && (
            <button
              onClick={() => setShowQualityDetails(!showQualityDetails)}
              className="mt-1 flex items-center justify-between text-[10px] font-medium text-sky-600 dark:text-sky-400 hover:underline cursor-pointer"
            >
              <span>{findings.length} Finding(s)</span>
              {showQualityDetails ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
            </button>
          )}

          {showQualityDetails && findings.length > 0 && (
            <div className="mt-2 space-y-2 text-[11px] border-t border-stroke/10 pt-2">
              {findings.map((f, i) => (
                <div key={i} className="p-2 rounded bg-amber-500/5 border border-amber-500/10 text-amber-800 dark:text-amber-200">
                  <div className="font-semibold">{f.message || f.recommendation}</div>
                  {f.recommendation && f.message && (
                    <div className="text-[10px] opacity-90 mt-0.5">Rec: {f.recommendation}</div>
                  )}
                  {f.file && (
                    <div className="text-[9px] font-mono mt-1 opacity-75">
                      {f.file}:{f.line ?? 1}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
