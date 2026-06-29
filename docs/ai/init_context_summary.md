# Context Initialization Summary: my_projects-auto_code_os

The project context initialization has completed successfully. Below is the structured profile of the project along with the designated Slot Map for AI skills.

## 📋 Project Identity & Stack
- **Project Name:** Auto Code OS (`my_projects-auto_code_os`)
- **Repository Location:** `/home/ubuntu/my_projects/auto_code_os`
- **Type:** Monorepo (AI-Native SDLC Platform)
- **Primary Tech Stack:** Go 1.26+ (Chi Router, GORM) & Next.js 16 (App Router, Tailwind CSS v4, TypeScript, React 19)
- **Database:** PostgreSQL 17 + pgvector (Migrations via `golang-migrate`)

## 🛠️ SLOT_MAP - Active & Suggested Skills

These skills have been successfully mapped to manage tasks in this repository efficiently:

| Slot Type | Skill Name | Purpose / Domain |
| :--- | :--- | :--- |
| **Core** | `project-memory` | Maintain persistent project-level context files in `docs/ai/` |
| **Core** | `context-management` | Keep context window slim and maintain token efficiency |
| **Core** | `clean-code` | Code quality, design token adherence, and readability |
| **Core** | `systematic-debugging` | Resolve issues and debug tests logically |
| **Core** | `verification-before-completion` | Ensure rigorous verification and validation before finishing |
| **Tech** | `golang-best-practices` | Go backend development, architecture, Chi routing, and GORM |
| **Tech** | `nextjs-best-practices` | Next.js App Router, page structures, and frontend code |
| **Tech** | `react-patterns` | State management, component optimization, React 19 rules |
| **Tech** | `typescript-expert` | Enforce TypeScript types, matching frontend to Go models |
| **Tech** | `tailwind-patterns` | Tailwind CSS v4 layout styling, CSS design tokens |
| **Tech** | `testing-patterns` | Maintain Go testing suite and Playwright E2E testing |

---

## 🏛️ Architecture & Domain Invariants
1. **Migration Strictness:** Migration files MUST follow strict sequential numbering to prevent `golang-migrate` conflicts.
2. **Layered Architecture:** Handlers must never call repositories directly. All communication flows: `internal/handler` ➔ `internal/service` ➔ `internal/repository`.
3. **Immutability of System Prompt:** Global guidelines and classifier rules always take precedence over task/project-level rules.
4. **Sandboxed Isolation:** All agent-triggered execution tasks must run inside isolated Docker containers using the Sandbox client.
5. **Human-in-the-Loop (HITL) Gate:** PR reviews and approvals are required for Medium and Hard tasks before merging.
