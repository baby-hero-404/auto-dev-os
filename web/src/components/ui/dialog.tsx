import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { cn } from "@/lib/cn";

export interface DialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  size?: "sm" | "md" | "lg";
  dismissable?: boolean;
  children: React.ReactNode;
  footer?: React.ReactNode;
}

export function Dialog({
  open,
  onClose,
  title,
  description,
  size = "md",
  dismissable = true,
  children,
  footer,
}: DialogProps) {
  const sizeClasses = {
    sm: "max-w-sm",
    md: "max-w-md",
    lg: "max-w-lg",
  };

  return (
    <DialogPrimitive.Root open={open} onOpenChange={(val) => {
      if (!val && dismissable) {
        onClose();
      }
    }}>
      <DialogPrimitive.Portal>
        {/* Overlay */}
        <DialogPrimitive.Overlay
          className="fixed inset-0 z-30 bg-slate-950/80 backdrop-blur-sm animate-fade-in"
        />
        {/* Content Container */}
        <div className="fixed inset-0 z-30 overflow-y-auto flex items-center justify-center p-4">
          <DialogPrimitive.Content
            className={cn(
              "w-full animate-modal-in rounded-xl border border-stroke bg-card p-6 shadow-xl relative",
              sizeClasses[size]
            )}
            onEscapeKeyDown={(event) => {
              if (!dismissable) {
                event.preventDefault();
              }
            }}
            onPointerDownOutside={(event) => {
              if (!dismissable) {
                event.preventDefault();
              }
            }}
          >
            {/* Header */}
            <div className="flex flex-col gap-1.5 mb-4 pr-6">
              <DialogPrimitive.Title className="text-sm font-semibold tracking-tight uppercase font-mono text-foreground">
                {title}
              </DialogPrimitive.Title>
              {dismissable && (
                <button
                  onClick={onClose}
                  className="absolute right-4 top-4 text-muted hover:text-foreground p-1 rounded-md transition-colors hover:bg-stroke"
                  aria-label="Close dialog"
                >
                  <X className="h-4 w-4" />
                </button>
              )}
              {description && (
                <DialogPrimitive.Description className="text-xs text-muted leading-normal">
                  {description}
                </DialogPrimitive.Description>
              )}
            </div>

            {/* Main content */}
            <div className="text-sm text-foreground/90">{children}</div>

            {/* Footer */}
            {footer && (
              <div className="flex items-center justify-end gap-3 mt-6 border-t border-stroke pt-4">
                {footer}
              </div>
            )}
          </DialogPrimitive.Content>
        </div>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
