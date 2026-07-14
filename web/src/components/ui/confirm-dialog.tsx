"use client";

import { AlertTriangle } from "lucide-react";
import { Dialog } from "./dialog";
import { Button } from "./button";

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
  const iconColors = {
    danger: "text-danger bg-danger/10 border-danger/20",
    warning: "text-warning bg-warning/10 border-warning/20",
    info: "text-info bg-info/10 border-info/20",
  };

  return (
    <Dialog
      open={isOpen}
      onClose={onClose}
      title={title}
      size="sm"
      dismissable={!isLoading}
      footer={
        <>
          <Button
            variant="secondary"
            onClick={onClose}
            disabled={isLoading}
            size="sm"
          >
            {cancelText}
          </Button>
          <Button
            variant={variant === "danger" ? "danger" : "primary"}
            className={
              variant === "warning"
                ? "bg-warning text-slate-950 hover:bg-warning/90 border-0"
                : variant === "info"
                ? "bg-brand-primary text-brand-primary-fg hover:opacity-90 border-0"
                : ""
            }
            onClick={async () => {
              if (isLoading) return;
              await onConfirm();
            }}
            isLoading={isLoading}
            size="sm"
          >
            {confirmText}
          </Button>
        </>
      }
    >
      <div className="flex gap-4 items-start py-2">
        <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full border ${iconColors[variant]}`}>
          <AlertTriangle size={20} aria-hidden="true" />
        </div>
        <div className="flex-1 text-sm text-content-muted leading-relaxed">
          {description}
        </div>
      </div>
    </Dialog>
  );
}
