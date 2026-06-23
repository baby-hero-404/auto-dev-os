export const getRiskAssessment = (complexity: string, files: string[], riskDomains?: string[]) => {
  const fileCount = files.length;
  let hasMigration = false;
  let hasConfig = false;
  for (const f of files) {
    const lower = f.toLowerCase();
    if (lower.includes("migration/") || lower.includes(".sql")) hasMigration = true;
    if (lower.includes("config") || lower.includes(".env") || lower.includes("docker")) hasConfig = true;
  }

  // Check risk domains
  let hasHighRiskDomain = false;
  if (riskDomains && riskDomains.length > 0) {
    const highRisk = ["auth", "payment", "security", "infra", "rbac", "permission"];
    for (const d of riskDomains) {
      if (highRisk.includes(d.toLowerCase())) {
        hasHighRiskDomain = true;
      }
    }
  }

  if (hasMigration && complexity === "hard") {
    return { level: "critical", reason: "Database migration in a hard-complexity task requires careful review" };
  }
  if (hasHighRiskDomain && complexity === "hard") {
    return { level: "critical", reason: "Modifying high-risk domains in a hard-complexity task requires extreme caution" };
  }
  if (hasMigration) {
    return { level: "high", reason: "Contains database migration files" };
  }
  if (hasHighRiskDomain) {
    return { level: "high", reason: "Modifies high-risk security, authentication, or payment systems" };
  }
  if (complexity === "hard" || fileCount > 15) {
    return { level: "high", reason: `Hard complexity task affecting ${fileCount} files` };
  }
  if (hasConfig) {
    return { level: "medium", reason: "Modifies configuration or infrastructure files" };
  }
  if (complexity === "medium" || fileCount > 5) {
    return { level: "medium", reason: `Medium complexity task affecting ${fileCount} files` };
  }
  return { level: "low", reason: `Simple change affecting ${fileCount} files` };
};
