# Technical Debt & Implementation Gap Report

**Date:** June 12, 2026
**Target:** `docs/ROADMAP.md` (Specifically Section 9) vs Current `auto_code_os` Implementation

This report highlights architectural discrepancies between the latest Roadmap vision and the current codebase state, focusing on the newly introduced Role-Based Skills and Centralized Repositories pattern. It also flags unused files created during recent UI refactoring efforts.

---

## Feature 1: Agent-Skill Decoupling (ROADMAP §9)

**Roadmap Mandate:**
*"Decoupled Agent-Skill Model: Agents are no longer statically bound to specific skills in the database. Instead, they are defined by their Role and Autonomy Level..."*

**Current Project Status:** ❌ **Out of Sync (Critical Debt)**

*   **Database & API:** The system still heavily relies on a static database join table (`agent_skills`). The `server/internal/repository/skill.go` explicitly has functions `ListByAgentID` and `AssignToAgent` which perform DB insertions tying agents to skills.
*   **API Routes:** The API client in `web/src/lib/api/index.ts` actively supports `assignSkillToAgent` and `listAgentSkills`.
*   **Frontend UI:**
    *   The project dashboard (`web/src/app/projects/[id]/page.tsx`) still maps `agentSkills` and provides a handler `handleAssignSkill` which it passes to the `SettingsTab`.
    *   `web/src/components/dashboard/settings-tab.tsx` still contains complex logic for manually assigning global skills to specific agents.

**Action Required:**
1.  Remove `assignSkillToAgent` and `agentSkills` data fetching from `web/src/app/projects/[id]/page.tsx` and `web/src/components/dashboard/settings-tab.tsx`.
2.  Deprecate the `agent_skills` table in the backend database.
3.  Remove `AssignToAgent` from `SkillService` and `SkillRepo`.

---

## Feature 2: Dynamic Skill Loading & Centralized Directory (ROADMAP §9)

**Roadmap Mandate:**
*"Centralized Skills Directory: A centralized directory is established at `skills/` parallel to the workspace (`auto_code_os/`) to organize skills cleanly: `skills/system/` and `skills/workspace/`"*

**Current Project Status:** ❌ **Not Implemented / Disconnected**

*   **Directory Structure:** The directory `/home/ubuntu/my_projects/skills/` does not currently exist, or the system is not actively mounting/reading from it.
*   **Execution Isolation:** The system still expects skills to be created via API (`POST /api/v1/skills` managed in `web/src/app/skills/page.tsx`?) rather than discovering markdown files dynamically from the filesystem.

**Action Required:**
1.  Initialize the `skills/system/` and `skills/workspace/` directories parallel to the workspace.
2.  Update `server/internal/orchestrator/prompt.go` (Sprint 3 in hardening plan) to dynamically load available tools/skills from the local filesystem instead of querying the `skills` table in the database.
3.  Convert existing database skills into filesystem Markdown files.

---

## Feature 3: Unused/Orphaned Files from Recent Refactor

Several new files were created to refactor the monolithic Project/Task UI, but these have not been fully wired or the legacy files have not been removed.

**Identified Files:**
1.  `web/src/lib/hooks/use-task-workflow.ts` (Partially used in new nested route)
2.  `web/src/lib/hooks/use-project-data.ts` (Partially used in new project route)
3.  `web/src/components/dashboard/spec-review-section.tsx` (Unused in legacy routes)
4.  `web/src/components/dashboard/log-console.tsx` (Unused in legacy routes)
5.  `web/src/lib/utils/tasks.ts` (Utility logic duplicated from `web/src/app/tasks/[id]/page.tsx`)

**Observation:**
While these files were created, the original monolithic task page `web/src/app/tasks/[id]/page.tsx` still exists and is over 700 lines long. The routing is currently fragmented between the old `/tasks/[id]` and the new nested `/projects/[id]/tasks/[taskID]`.

**Action Required:**
1.  Finalize the migration of logic out of `web/src/app/tasks/[id]/page.tsx` into the new components and hooks.
2.  Delete `web/src/app/tasks/[id]/page.tsx` to force all traffic through the project-scoped `/projects/[id]/tasks/[taskID]` route.
3.  Ensure `web/src/app/projects/[id]/tasks/[taskID]/monitor/page.tsx` uses the new `useTaskWorkflow.ts` hook instead of duplicating SWR logic.

---

## Feature 4: Agent System Auto-Join Hook (ROADMAP §4.3 Backlog)

**Roadmap Mandate:**
*"Auto-join Agent khi Project mới được tạo"* (Auto-join Agent when a new Project is created).

**Current Project Status:** ⚠️ **Working via JOIN, Missing Audit Trail**

*   According to `docs/backlog/agent-system-unimplemented.md`, agents with `assignment_strategy = 'auto_join'` successfully appear in projects due to a SQL `JOIN` clause in `ListByProjectID`, but they are not physically written into the `project_agents` table upon project creation.

**Action Required:**
1.  Decide whether to keep the JOIN-only approach or inject a hook into `ProjectService.Create()` to physically materialize the `project_agents` links for better auditability.

---

## Next Steps Plan

1.  **UI Cleanup Sprint:** Rip out the static skill assignment UI from the `SettingsTab` and remove `agent_skills` queries to align with Roadmap Update 9.
2.  **Route Consolidation Sprint:** Delete the legacy `/tasks/[id]` route and fully integrate the newly created `use-task-workflow` and `use-project-data` hooks into the nested project view.
3.  **Backend Skill Migration Sprint:** Shift skill reading from DB queries to filesystem reads targeting `../skills/system` and `../skills/workspace`.
