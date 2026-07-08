import React from "react";

export function Field({
  label,
  action,
  children,
}: {
  label: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <div className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted/80">
          {label}
        </div>
        {action}
      </div>
      {children}
    </div>
  );
}
