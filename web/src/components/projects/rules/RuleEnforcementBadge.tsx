import { Badge, ruleEnforcementBadge } from "@/components/ui/badge";
import type { Rule } from "@/lib/types";

interface RuleEnforcementBadgeProps {
  enforcement: Rule["enforcement"];
}

export function RuleEnforcementBadge({ enforcement }: RuleEnforcementBadgeProps) {
  const meta = ruleEnforcementBadge(enforcement);
  return (
    <Badge variant={meta.variant} className="font-mono text-[9px] font-bold uppercase tracking-wider px-2 py-0.5">
      {meta.label}
    </Badge>
  );
}
