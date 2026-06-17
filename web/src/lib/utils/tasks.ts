export const getRiskAssessment = (complexity: string, files: string[]) => {
  const fileCount = files.length;
  let hasMigration = false;
  let hasConfig = false;
  for (const f of files) {
    const lower = f.toLowerCase();
    if (lower.includes("migration/") || lower.includes(".sql")) hasMigration = true;
    if (lower.includes("config") || lower.includes(".env") || lower.includes("docker")) hasConfig = true;
  }

  if (hasMigration && complexity === "hard") {
    return { level: "critical", reason: "Database migration in a hard-complexity task requires careful review" };
  }
  if (hasMigration) {
    return { level: "high", reason: "Contains database migration files" };
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
