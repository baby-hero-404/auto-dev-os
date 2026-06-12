# Feature Specification: Skill System & Tool Isolation

> **Status:** Partially Implemented / Evolving
> **Last Updated:** 2026-06-12
> **Core Components:** `server/pkg/models/skill.go`, `server/internal/orchestrator/prompt.go`, `server/internal/orchestrator/skill_executor.go`, `web/src/app/skills/page.tsx`

---

## 1. Overview

In AutoCodeOS, a **Skill** is a reusable capability or body of knowledge that an Agent can use to execute tasks. Skills act as the primary mechanism for:
1. **Tool Isolation**: Restricting which tools an Agent can use based on its assigned Role, preventing LLM tool confusion.
2. **JIT Knowledge Protocol**: Providing Just-In-Time domain knowledge (e.g., React patterns, Golang best practices) dynamically, preventing context window saturation.

Instead of loading all organizational knowledge into the base system prompt, Agents are assigned specific Skills and discover their contents dynamically during workflow execution.

---

## 2. Skill Architecture

### 2.1 The Data Model

Skills are represented by a database record (`models.Skill`) that maps to a physical Markdown file.

| Field | Type | Description |
|---|---|---|
| `ID` | UUID | Primary key |
| `Name` | string | Unique identifier (e.g., `react-patterns`, `golang-best-practices`) |
| `Description`| string | Brief summary of what the skill provides |
| `Schema` | JSON | Holds metadata: `{"source": "prompt_base", "category": "tech", "path": "antigravity/skills/tech/react-patterns/SKILL.md", "allowed_tools": ["read_file", "write_file", "apply_patch"]}` |

### 2.2 The File Format (Markdown + YAML Frontmatter)

While the database holds the metadata, the actual knowledge and instructions live in physical files (typically `SKILL.md`). These files use YAML frontmatter:

```markdown
---
name: react-patterns
description: "Use when building React applications, working with Hooks."
allowed-tools: [read_file, write_file, apply_patch, search_code]
---
# React Patterns
> Principles for building production-ready React applications...
```

#### 2.2.1 Frontmatter Field Reference

| Field | Required | Format | Description |
|---|---|---|---|
| `name` | Yes | `kebab-case` string | Canonical skill identifier. Must match `^[a-z0-9][a-z0-9\-_]*$`. |
| `description` | No | Quoted string | One-line summary injected into agent prompt context. |
| `allowed-tools` | No | List of system tool keys | Exact tool identifiers from the builtin registry (see §3.1.1). Falls back to `[read_file, write_file]` if omitted. |

---

## 3. Core Mechanisms

### 3.1 Tool Isolation (Preventing Tool Overload)

By default, LLMs can get confused if presented with 20+ tools simultaneously. AutoCodeOS uses Skills to implement **Tool Isolation**:

1. Each Agent is assigned specific Skills (e.g., `Frontend Specialist` gets `react-patterns`).
2. The Orchestrator's `PromptAssembler` reads the `Schema` of the Agent's assigned Skills.
3. The `PromptAssembler` extracts `allowed_tools` from the schema and **filters the global tool list** via `FilterToolsBySkills()`.
4. The Agent only sees the tools explicitly permitted by its assigned Skills.
5. **Security Validation:** The `skill_executor.go` → `callAllowsTool()` verifies tool execution requests against the Agent's authorized tool list at runtime.

#### 3.1.1 Builtin Tool Registry

The following tool keys are registered in `BuiltinToolDefinitions()`:

| Tool Key | Purpose |
|---|---|
| `read_file` | Read a workspace file (max 12KB default) |
| `write_file` | Write content to a workspace file |
| `apply_patch` | Token-efficient search-and-replace edit |
| `search_code` | Search source files for a literal string |
| `run_tests` | Run project tests inside the sandbox |
| `analyze_logs` | Read and summarize log files |
| `generate_docs` | Generate documentation text |
| `create_migration` | Create SQL migration draft |
| `read_skill` | Fetch assigned skill content by name *(planned — see §3.2.1)* |

#### 3.1.2 Tool Name Resolution & Alias Map

Frontmatter `allowed-tools` values undergo normalization before matching:

1. **Normalization:** Hyphens and spaces are converted to underscores, lowercased.
2. **Alias Resolution:** Human-readable shorthand names resolve to system tool keys:

| Alias | Resolves To |
|---|---|
| `read` | `read_file` |
| `write` | `write_file` |
| `edit`, `patch` | `apply_patch` |
| `test`, `tests` | `run_tests` |
| `search` | `search_code` |
| `docs` | `generate_docs` |
| `migrate` | `create_migration` |
| `logs` | `analyze_logs` |

3. **Category Fallback:** If the Schema contains a `category` field, `addToolsForCategory()` grants a default tool set for that category (e.g., `"code"` → `read_file, write_file, search_code, apply_patch`).

> **Best Practice:** Use exact system tool keys in frontmatter for clarity. Aliases exist for backward compatibility.

### 3.2 JIT Knowledge Discovery

The actual Markdown content of a skill is **not** automatically injected into the Agent's context.
Instead:

1. The Orchestrator injects a list of available Skills (names and descriptions) into the prompt.
2. If the Agent encounters a task requiring specific knowledge, it fetches the skill content on demand.
3. Once the subtask is complete, the knowledge is pruned from the context window.

#### 3.2.1 Skill Content Access Strategy: `read_skill` Tool

**Problem:** Custom skill files are saved on the host filesystem, but agent execution occurs inside isolated Docker sandboxes. Volume-mounting host paths introduces security risks and breaks in remote/cloud sandbox environments.

**Solution:** Introduce a `read_skill` built-in tool that serves skill content through the orchestrator without requiring direct filesystem access from the sandbox:

```
Agent calls: read_skill(skill_name: "react-patterns")
  → Orchestrator resolves Schema.path from the DB
  → Reads the file on the host
  → Returns Markdown content to the agent
  → Agent receives skill knowledge without FS access
```

**Authorization Rule:** The agent MUST have the skill assigned to read its content. Unassigned skill reads are rejected with an authorization error.

**Fallback (interim):** Until `read_skill` is implemented, the sandbox mounts `~/.gemini/antigravity/skills/` as a read-only volume at `/opt/auto-code-os/skills/` and agents use `read_file` with the relative path. This approach is acceptable for local development only.

---

## 4. Centralized Skills Repository & Seeding Workflow

To support clean decoupling between workspace code and system capabilities, the system stores and tracks agent skills within a centralized directory structure situated outside individual projects.

### 4.1 Centralized Skills Structure

The skills repository resides in a root `skills/` folder parallel to the active project workspace (e.g. `/home/ubuntu/my_projects/skills` next to `/home/ubuntu/my_projects/auto_code_os`):

```
my_projects/
├── auto_code_os/           # Active project workspace
└── skills/                 # Centralized skills root directory
    ├── system/             # System-wide (global) agent skills
    │   ├── core/
    │   ├── custom/
    │   ├── process/
    │   └── tech/
    └── workspace/          # Project-specific / workspace-specific skills
```

- **`skills/system/`**: Holds core, process, custom, and tech skills shared system-wide across all workspaces and agents.
- **`skills/workspace/`**: Isolated directory reserved for skills generated or customized specifically for the active project/workspace.

### 4.2 Configured Skills Root
The root directory of the skills system is configured in the server's configuration file (`config.yaml`) under the `sandbox` section:

```yaml
sandbox:
  workspace_root: "/tmp/auto-code-os/workspaces"
  skills_root: "/home/ubuntu/my_projects/skills"
```

In Go code, this corresponds to the `SkillsRoot` field in `SandboxConfig`.

### 4.3 Dynamic Loading & Discovery Workflow

The orchestrator and services dynamically discover and seed skills using this folder setup:

1. **Initial Seeding**: The `SeederService` reads the default skills metadata from `skills_root/registry.min.json` (falling back to `resources/prompt_base/registry.min.json` if not configured) and imports them into PostgreSQL.
2. **File Paths Resolution**: 
   - System skills from `prompt_base` are automatically mapped to `system/<category>/<skill_name>` (e.g. `system/tech/react-patterns`).
   - Project-specific or custom-uploaded skills are mapped to `workspace/<skill_name>`.
3. **Execution Mounting**: The orchestrator's sandbox environment reads custom skills via the secure API endpoints or maps the paths from the centralized directory without exposing host paths.

---

## 5. Planned Enhancement: Bulk Custom Import

To support bringing custom skills into the platform via the UI (without manually editing `registry.min.json` on the server), the system implements an **HTML5 File Upload Workflow**.

### 5.1 Upload Flow

```
User selects .md files → Browser sends multipart/form-data
  → POST /api/v1/skills/upload (form field: "files")
  → Backend iterates each file:
      1. Parse YAML frontmatter (name, description, allowed-tools)
      2. Sanitize & validate name
      3. Write .md file to ~/.gemini/antigravity/skills/custom/<name>.md
      4. Upsert Skill record in PostgreSQL
  → Response: array of created/updated Skill objects
```

### 5.2 Safety & Validation Rules

#### 5.2.1 Path Traversal Prevention

All uploaded filenames and parsed frontmatter names MUST be sanitized before use:

```
1. Strip directory components: name = filepath.Base(name)
2. Enforce character whitelist: name = regexp.MustCompile(`[^a-z0-9\-_]`).ReplaceAllString(name, "")
3. Reject empty names after sanitization
4. Verify final write path does not escape the target directory
```

#### 5.2.2 Malformed Frontmatter Fallback

If YAML frontmatter parsing fails or required fields are missing:

| Missing Field | Fallback Behavior |
|---|---|
| `name` | Derive from filename: `my-skill.md` → `my-skill` |
| `description` | Set to empty string `""` |
| `allowed-tools` | Default to `[]` (agent receives `[read_file, write_file]` via FilterToolsBySkills fallback) |
| Entire frontmatter | Treat as name-from-filename with default schema `{"source": "custom", "allowed_tools": []}` |

#### 5.2.3 De-duplication & Conflict Resolution

When an uploaded skill name collides with an existing database record:

| Scenario | Behavior |
|---|---|
| Custom overwrites custom | **Update** — overwrite description, schema, and file content |
| Custom collides with seeded (prompt_base) | **Update** — custom takes precedence, `source` field changes to `custom` |
| Re-seed after custom override | **Skip** — seeder skips names that already exist (existing behavior) |

All overwrites MUST be logged to the audit trail:
```
AuditService.RecordAction(ctx, agentID, "skill.overwrite", skillName)
```

### 5.3 Upload Endpoint Configuration

| Parameter | Value |
|---|---|
| **Route** | `POST /api/v1/skills/upload` |
| **Auth** | Required (JWT) |
| **Max Form Size** | 10 MB |
| **Timeout Override** | 60 seconds (override the default 30s chi middleware for this route) |
| **Accepted Files** | `.md` files only; reject other extensions |
| **Form Field Name** | `files` (multiple) |

---

## 6. Implementation Checklist

### Phase A: Tool Name Resolution (Low Effort)

- [ ] Add `toolAliases` map to `prompt.go` → `addAllowedTool()`
- [ ] Update `addAllowedTool()` to resolve aliases before matching against known tools
- [ ] Add unit tests for alias resolution edge cases
- [ ] Update frontmatter examples in this spec and any seed data to use exact tool keys

### Phase B: `read_skill` Tool (Medium Effort)

- [ ] Add `read_skill` to `BuiltinToolDefinitions()` in `skill_executor.go`
- [ ] Implement `readSkill()` executor method: resolve path from DB, verify agent assignment, read and return content
- [ ] Wire `SkillRepo` dependency into `SkillExecutor`
- [ ] Add unit tests for authorized/unauthorized/missing skill reads
- [ ] Remove interim volume mount once `read_skill` is stable

### Phase C: Bulk Custom Import (Medium Effort)

- [ ] Implement `ImportCustom()` in `service/skill.go` with sanitization, file write, and upsert
- [ ] Implement `parseFrontmatter()` utility with fallback logic
- [ ] Add `Upload()` handler in `handler/skill.go` with multipart parsing
- [ ] Register `POST /skills/upload` route in `router.go` with 60s timeout override
- [ ] Update `api/client.ts` → `request()` to skip `Content-Type: application/json` for `FormData` bodies
- [ ] Add `skills.upload()` method to `web/src/lib/api/agents.ts`
- [ ] Add upload button to `web/src/app/skills/page.tsx` (file picker, progress toast)
- [ ] Add audit logging for all import/overwrite events
- [ ] Write Go unit tests for `ImportCustom`, `parseFrontmatter`, path traversal rejection
