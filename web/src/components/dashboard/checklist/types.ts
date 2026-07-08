import type { LucideIcon } from "lucide-react";

export type CheckItem = {
  id: string;
  label: string;
  href: string;
  hrefLabel: string;
  icon: LucideIcon;
  required: boolean;
  done: boolean;
  onClick?: () => void;
};
