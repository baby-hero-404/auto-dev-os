import type { Agent } from "@/lib/types";
import { DEFAULT_FLEET } from "./DEFAULT_FLEET";

export function providerSummary(agents: Agent[]) {
  const counts = agents.reduce<Record<string, number>>((acc, agent) => {
    const levelGroup = agent.model_level_group || "";
    const provider = levelGroup.includes("/") ? levelGroup.split("/")[0] : "gateway";
    acc[provider] = (acc[provider] || 0) + 1;
    return acc;
  }, {});
  return Object.entries(counts).map(([provider, count]) => `${provider} (${count})`).join(", ");
}

export function seedFleetTitle(agentCount: number) {
  if (agentCount >= DEFAULT_FLEET.length) return "Fleet already seeded";
  if (agentCount > 0) return `${agentCount} of ${DEFAULT_FLEET.length} agents created - click to retry`;
  return "Create the default seven-agent fleet";
}
