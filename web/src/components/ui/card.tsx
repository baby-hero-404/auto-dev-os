import * as React from "react";
import { cn } from "@/lib/cn";

export type CardProps = React.HTMLAttributes<HTMLDivElement>;

export function Card({ className, ...props }: CardProps) {
  return (
    <div
      className={cn(
        "rounded-lg border border-stroke bg-card p-5 text-card-fg shadow-sm",
        className
      )}
      {...props}
    />
  );
}

export interface CardHeaderProps extends React.HTMLAttributes<HTMLDivElement> {
  title: string;
  icon?: React.ReactNode;
  action?: React.ReactNode;
}

export function CardHeader({
  className,
  title,
  icon,
  action,
  ...props
}: CardHeaderProps) {
  return (
    <div
      className={cn("flex items-center justify-between gap-4 mb-4", className)}
      {...props}
    >
      <div className="flex items-center gap-2">
        {icon && <div className="text-muted flex items-center justify-center">{icon}</div>}
        <h3 className="text-sm font-semibold text-foreground tracking-tight uppercase font-mono">
          {title}
        </h3>
      </div>
      {action && <div>{action}</div>}
    </div>
  );
}

export type CardContentProps = React.HTMLAttributes<HTMLDivElement>;

export function CardContent({ className, ...props }: CardContentProps) {
  return <div className={cn("text-sm text-foreground/90", className)} {...props} />;
}
