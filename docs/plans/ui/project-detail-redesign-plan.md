# Project Detail Page — UI/UX Redesign Plan

## Design Direction

Inspired by **Linear**, **Vercel Dashboard**, and **GitHub Projects** — moving from a monolithic tabbed layout to a **sidebar-driven navigation** with focused, single-purpose views.

## Mockups

### Tasks View (Main)
![Tasks View](/home/ubuntu/.gemini/antigravity/brain/5447565e-a594-45be-9b64-a9e94a2b2f0a/project_detail_redesign_1781261012810.png)

### Settings View
![Settings View](/home/ubuntu/.gemini/antigravity/brain/5447565e-a594-45be-9b64-a9e94a2b2f0a/settings_redesign_1781261043717.png)

---

## Key Design Changes

### 1. Layout: Tabs → Sidebar Navigation

**Before:** Single `<section>` with 2 tabs (Tasks, Settings)
**After:** Persistent left sidebar with 5 focused views

| Sidebar Item | Content | Why Separated |
|---|---|---|
| **Tasks** | Task list + filters + status summary | Primary workflow — deserves full width |
| **Agents** | Agent grid + skill assignment | Agents are a first-class concept, not buried in settings |
| **Repositories** | Repo list + link form | Critical for task context, hidden before |
| **Rules** | Rule list + add/seed | Behavioral config deserves dedicated space |
| **Settings** | Project name/description only | Clean, minimal, rarely visited |

### 2. Tasks View: Add Filtering + Search

- **Filter chips**: All / Active / Review / Failed — quick status filtering
- **Search bar**: Filter tasks by title/description
- **Compact task rows**: Status dot → Title → Complexity badge → Status badge → Agent initials → Time ago
- **Inline actions**: Analyze/Execute buttons directly on hover

### 3. Status Summary: Horizontal Strip

- Compact metric cards in a single row above the task list
- Active Tasks • Needs Review • Failed • Agents Available
- Color-coded numbers for quick scanning

### 4. Visual Refinements

- Remove `font-mono` from headings (use Inter for readability)
- Use consistent card surfaces with `glass-panel` class
- Better hover states with accent glow
- Agent initials avatars instead of text labels
- Proper empty states with illustrations

---

## Architecture Changes

```
Before:
page.tsx → SettingsTab (22 props!) → 5 sub-components

After:
page.tsx → ProjectSidebar (navigation only)
        → {activeView} renders one focused view at a time
        → Each view fetches its own data (no prop drilling)
```

### Files to Create/Modify

| Action | File | Purpose |
|--------|------|---------|
| **Create** | `project-sidebar.tsx` | Left navigation sidebar |
| **Create** | `project-header.tsx` | Breadcrumb + action buttons |
| **Create** | `task-filters.tsx` | Search + filter chips |
| **Rewrite** | `page.tsx` | Sidebar layout orchestrator |
| **Refactor** | `tasks-tab.tsx` | Add filters, improve task rows |
| **Refactor** | `project-status-summary.tsx` | Horizontal compact strip |
| **Simplify** | `settings-tab.tsx` | Only project metadata form |
| **Promote** | `members-tab.tsx` → `agents-view.tsx` | Standalone agents view |
| **Promote** | `repo-sidebar.tsx` → `repositories-view.tsx` | Standalone repos view |
| **Promote** | `rules-manager.tsx` → `rules-view.tsx` | Standalone rules view |

---

## Implementation Phases

### Phase 1: Layout Foundation
- Create `ProjectSidebar` + `ProjectHeader`
- Restructure `page.tsx` with sidebar layout
- 5-view switching mechanism

### Phase 2: Tasks View Enhancement
- Add `TaskFilters` component
- Redesign task rows (compact, Linear-style)
- Improve empty state

### Phase 3: Promote Sub-Sections to Views
- Agents, Repositories, Rules as standalone views
- Each fetches its own data (eliminate prop drilling)

### Phase 4: Polish
- Transitions between views
- Keyboard shortcuts (1-5 for view switching)
- Mobile responsive collapse of sidebar

---

> [!IMPORTANT]
> **Awaiting your approval to proceed with implementation.**
> I'll implement all 4 phases in sequence, starting with the layout foundation.
