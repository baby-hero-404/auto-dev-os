"use client";

import { useEffect, useRef } from "react";
import { AlertTriangle, X, Loader2 } from "lucide-react";

interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  description: string;
  confirmText?: string;
  cancelText?: string;
  variant?: "danger" | "warning" | "info";
  isLoading?: boolean;
  onConfirm: () => void | Promise<void>;
  onClose: () => void;
}

export function ConfirmDialog({
  isOpen,
  title,
  description,
  confirmText = "Confirm",
  cancelText = "Cancel",
  variant = "danger",
  isLoading = false,
  onConfirm,
  onClose,
}: ConfirmDialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const confirmBtnRef = useRef<HTMLButtonElement>(null);
  const cancelBtnRef = useRef<HTMLButtonElement>(null);

  // Close on Escape, Trap Focus
  useEffect(() => {
    if (!isOpen) return;

    // Focus initial focus target on mount
    if (cancelBtnRef.current) {
      cancelBtnRef.current.focus();
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        if (!isLoading) {
          onClose();
        }
        return;
      }

      if (e.key === "Tab" && dialogRef.current) {
        const focusableElements = dialogRef.current.querySelectorAll(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        if (focusableElements.length === 0) return;
        const firstElement = focusableElements[0] as HTMLElement;
        const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

        if (e.shiftKey) {
          if (document.activeElement === firstElement) {
            lastElement.focus();
            e.preventDefault();
          }
        } else {
          if (document.activeElement === lastElement) {
            firstElement.focus();
            e.preventDefault();
          }
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose, isLoading]);

  if (!isOpen) return null;

  const variantColors = {
    danger: "bg-red-500 hover:bg-red-600 text-white focus:ring-red-500",
    warning: "bg-amber-500 hover:bg-amber-600 text-slate-950 focus:ring-amber-500",
    info: "bg-brand-primary hover:opacity-90 text-slate-950 focus:ring-brand-primary",
  };

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center p-4 animate-fade-in"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-title"
      aria-describedby="confirm-desc"
      ref={dialogRef}
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-slate-950/80 backdrop-blur-sm transition-opacity duration-300"
        onClick={() => {
          if (!isLoading) onClose();
        }}
        aria-hidden="true"
      />

      {/* Modal Dialog Card */}
      <div className="animate-modal-in relative z-10 flex w-full max-w-md flex-col rounded-xl border border-stroke bg-card shadow-2xl overflow-hidden p-6">
        <button
          onClick={onClose}
          disabled={isLoading}
          className="absolute top-4 right-4 rounded-md p-1.5 text-content-muted transition-colors hover:bg-surface hover:text-foreground cursor-pointer disabled:opacity-50"
          type="button"
          aria-label="Close dialog"
        >
          <X size={16} aria-hidden="true" />
        </button>

        <div className="flex gap-4 items-start">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-red-500/10 text-red-500 border border-red-500/20">
            <AlertTriangle size={20} aria-hidden="true" />
          </div>

          <div className="flex-1">
            <h3 id="confirm-title" className="font-sans text-base font-bold text-foreground">{title}</h3>
            <p id="confirm-desc" className="mt-2 text-sm text-content-muted leading-relaxed">{description}</p>
          </div>
        </div>

        <div className="mt-6 flex items-center justify-end gap-3 shrink-0">
          <button
            onClick={onClose}
            disabled={isLoading}
            ref={cancelBtnRef}
            className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer focus:outline-none focus:ring-2 focus:ring-stroke-focus focus:ring-offset-2 disabled:opacity-50"
            type="button"
          >
            {cancelText}
          </button>
          <button
            ref={confirmBtnRef}
            className={`inline-flex items-center gap-1.5 rounded-md px-4 py-2 text-sm font-semibold transition cursor-pointer focus:outline-none focus:ring-2 focus:ring-offset-2 ${variantColors[variant]} disabled:opacity-50`}
            onClick={async () => {
              if (isLoading) return;
              await onConfirm();
            }}
            disabled={isLoading}
            type="button"
          >
            {isLoading && <Loader2 size={14} className="animate-spin text-current" />}
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
