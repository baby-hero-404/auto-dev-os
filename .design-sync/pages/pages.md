# Page references

Full-screen captures of every real route in the app, rendered with
representative mock data. These are **not** design-system components — they
don't mount from `window.AutoCodeOsUI` and have no props, variants, or
`.d.ts`. They exist so a human or the design agent can see what a whole page
actually looks like when the components in this kit are composed together in
context (layout, spacing, real copy, empty/loading states as they occur in
practice).

Each page is a thin `DashboardLayout` wrapper around one or more
data-bound feature components — not itself a reusable piece — so unlike the
`components/` cards these are reference-only: don't copy markup out of them
expecting it to be a drop-in component.

| Page | Route | Image |
| --- | --- | --- |
| Home / Login | `/` | `pages/Home.png` |
| Agents | `/agents` | `pages/Agents.png` |
| AI Providers | `/ai-providers` | `pages/AI-Providers.png` |
| Analytics | `/analytics` | `pages/Analytics.png` |
| Audit | `/audit` | `pages/Audit.png` |
| Gateway | `/gateway` | `pages/Gateway.png` |
| Git Accounts | `/git-accounts` | `pages/Git-Accounts.png` |
| Knowledge | `/knowledge` | `pages/Knowledge.png` |
| Knowledge Suggestions | `/knowledge/suggestions` | `pages/Knowledge-Suggestions.png` |
| Organization | `/organization` | `pages/Organization.png` |
| Rules | `/rules` | `pages/Rules.png` |
| Settings | `/settings` | `pages/Settings.png` |
| Skills | `/skills` | `pages/Skills.png` |
| Project Workspace | `/projects/[id]` | `pages/Project-Workspace.png` |
| Task Detail | `/projects/[id]/tasks/[taskID]` | `pages/Task-Detail.png` |

Captured via Playwright against the real app with the repo's own e2e mock
fixtures (`web/e2e/fixtures/api-mocks.ts`), full-page, at the default
viewport. Excluded: pure-redirect stubs with no rendered UI of their own
(e.g. the legacy `/tasks/[id]` route, which just forwards into the
project-scoped task URL).
