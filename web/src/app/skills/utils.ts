import type { Skill } from "@/lib/types";

export type SkillMeta = {
  repo: string;
  category: string;
  registry: string;
  path: string;
  source: string;
};

export function parseSkillMeta(skill: Skill): SkillMeta {
  const fallback = {
    repo: "unknown",
    category: "general",
    registry: skill.id,
    path: "",
    source: "registry",
  };

  try {
    const parsed = typeof skill.schema === "string" ? JSON.parse(skill.schema) : skill.schema;
    return {
      repo: typeof parsed?.repo === "string" ? parsed.repo : fallback.repo,
      category: typeof parsed?.category === "string" ? parsed.category : fallback.category,
      registry: typeof parsed?.registry === "string" ? parsed.registry : fallback.registry,
      path: typeof parsed?.path === "string" ? parsed.path : fallback.path,
      source: typeof parsed?.source === "string" ? parsed.source : fallback.source,
    };
  } catch {
    return fallback;
  }
}

export function repoNameFromURL(gitURL: string) {
  const cleaned = gitURL.trim().replace(/\.git$/, "");
  const parts = cleaned.split("/").filter(Boolean);
  return parts.at(-1) || "unknown";
}

export function cleanRepoPath(path: string) {
  const parts = path.split("/").filter(Boolean);
  if (parts[0] === "git" && parts.length > 1) {
    return parts.slice(2).join("/");
  }
  return path;
}

export function folderForSkill(skill: Skill) {
  const path = cleanRepoPath(parseSkillMeta(skill).path);
  if (!path) return "";
  if (path.endsWith(".md")) {
    const parts = path.split("/").filter(Boolean);
    if (parts.length <= 1) return "";
    return parts.slice(0, -1).join("/");
  }
  return path;
}

export function fileForSkill(skill: Skill) {
  const path = cleanRepoPath(parseSkillMeta(skill).path);
  if (!path) return "SKILL.md";
  if (path.endsWith(".md")) return path;
  return `${path}/SKILL.md`;
}

export function formatDateTime(value?: string) {
  if (!value) return "Never";
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

export function formatSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 KB";
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}

export function validateSkillRepoURL(rawURL: string) {
  const value = rawURL.trim();
  if (!value) {
    return "Repository URL is required.";
  }

  if (/\s/.test(value)) {
    return "Repository URL cannot contain spaces.";
  }

  if (/^[\w.-]+@[\w.-]+:[^/].+$/.test(value)) {
    return null;
  }

  try {
    const parsed = new URL(value);
    if ((parsed.protocol === "http:" || parsed.protocol === "https:") && (!parsed.hostname || !parsed.pathname || parsed.pathname === "/")) {
      return "Use a repository URL with an owner and repo path, for example https://github.com/org/repo.git.";
    }
    if ((parsed.protocol === "ssh:" || parsed.protocol === "git+ssh:") && (!parsed.hostname || !parsed.pathname || parsed.pathname === "/")) {
      return "Use an SSH repository URL with a host and repo path, for example ssh://git@github.com/org/repo.git.";
    }
    if (parsed.protocol === "file:" && !parsed.pathname) {
      return "Local file URLs must point to a repository path.";
    }
    if (!["http:", "https:", "ssh:", "git+ssh:", "file:"].includes(parsed.protocol)) {
      return "Use an HTTPS, SSH, git+ssh, or file:// repository URL.";
    }
    return null;
  } catch {
    return "Use an HTTPS, SSH, git+ssh, or file:// repository URL.";
  }
}
