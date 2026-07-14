# Project Pages UI/UX Redesign

## Context

The project-related pages (dashboard list, project detail, create modal, settings) in `web/` have accumulated visible inconsistencies and UX dead-ends:

- **Design bug:** primary buttons split ~50/50 between `text-white` and `text-slate-950` on the same `bg-brand-primary` — and contrast math shows *neither* is right in both themes (light accent `oklch(0.55 0.20 255)` needs white text; dark accent `oklch(0.65 0.20 255)` needs dark text).
- **No shared primitives:** no Button/Input/Card/Skeleton/Dialog — every form hand-rolls styling with drift. Radix UI, cva, clsx, tailwind-merge are all installed with **zero imports**.
- **Navigation dead-end:** project detail bypasses the global shell; breadcrumb "Projects" is a plain `<span>`; tabs are local state (reload loses position); keyboard shortcuts 1–5 undiscoverable.
- **Mobile nearly broken:** project sidebar is `hidden md:flex`; only nav is a small `<select>`; no back affordance.
- **Inconsistent states:** 4 hand-rolled empty states despite a shared `EmptyState`, 6 independent skeletons, divergent error treatments, raw color scales bypassing semantic tokens.

Style authority: `.design-sync/conventions.md` (semantic tokens, Inter/JetBrains Mono, theme radius, existing animation classes). `docs/design-system/autocodeos/MASTER.md` is stale and will be deprecated, not followed.

Goal: comprehensive but phased redesign — each phase shippable — that fixes the above while extending (not replacing) the existing OKLCH token system.

## Key design decisions

1. **Button text color → new per-theme token.** Add `--accent-fg` (`:root` → `oklch(1 0 0)`, `.dark` → `oklch(0.13 0.006 285.823)`) + `--color-brand-primary-fg` bridge in `@theme inline` → `text-brand-primary-fg`. Encoded once in the Button primitive; passes AA in both themes.
2. **Plain variant maps + `cn()` (clsx+twMerge), no cva** — matches the codebase's existing `Record<string,string>` idiom (`confirm-dialog.tsx`, `badge.tsx`).
3. **Tab routing via `?view=` query param** read with `useSearchParams()` (whitelist-validated, default `"tasks"`), navigated via real `<Link>`s; shortcuts use `router.replace(..., { scroll: false })`. Survives reload, shareable, least invasive for the fully client-side SWR data (rejected nested routes: would require layout.tsx + provider moves + 5 page files for no data benefit).
4. **Keep the custom two-pane workspace shell** (don't wrap in DashboardLayout — double chrome, breaks full-height workspace). Fix dead-ends instead: linked breadcrumb, home link in sidebar, mobile back chevron + horizontal tab strip.
5. **Slim ProjectHeader to identity + CTA**; count chips (duplicate sidebar badges) and WorkflowStageStrip (task-domain only) move into `ProjectStatusSummary` on the Tasks view.
6. **Badge v2:** semantic variants (`neutral|accent|success|warning|danger|info` + a few hue extras) + exported per-domain mappers (`taskStatusBadge`, `prStatusBadge`, `ruleEnforcementBadge`, `agentStatusBadge`, `projectStatusBadge`) — kills cross-domain key collisions and the silent gray fallback.
7. **Radix Dialog as the single overlay primitive** — portals to body, permanently fixing the transformed-ancestor centering bug documented in `.design-sync/NOTES.md`; deletes two hand-rolled focus traps. `ConfirmDialog` props stay identical.
8. **No generic Tabs primitive** — detail tabs become links; nothing else needs tabs yet.

## Phase 1 — Foundation: tokens + `ui/` primitives (Completed)

New files in `web/src/`:
- `lib/cn.ts` — `cn(...inputs: ClassValue[]): string` (clsx + tailwind-merge).
- `components/ui/button.tsx` — `variant: "primary"|"secondary"|"ghost"|"danger"` (default primary), `size: "sm"|"md"`, `isLoading`, `asChild` (via `@radix-ui/react-slot` for Link-as-button). Primary = `bg-brand-primary text-brand-primary-fg hover:opacity-90`; danger matches existing ConfirmDialog red (sanctioned by conventions.md).
- `components/ui/input.tsx`, `textarea.tsx`, `select.tsx` — thin native wrappers; canonical classes lifted from `project-profile.tsx`: `rounded-md border border-stroke bg-surface px-3 py-2 text-sm focus:border-brand-primary focus:outline-none disabled:opacity-50`.
- `components/ui/field.tsx` — `Field({ label, htmlFor, error?, hint?, children })` with existing mono-uppercase label style, errors in `text-danger`.
- `components/ui/card.tsx` — `Card`, `CardHeader({ icon?, title, action? })`, `CardContent`; base `rounded-lg border border-stroke bg-card p-5`.
- `components/ui/skeleton.tsx` — wraps existing `.skeleton-shimmer` utility.
- `components/ui/dialog.tsx` — Radix wrapper: `{ open, onClose, title, description?, size: sm|md|lg, dismissable?, children, footer? }`; overlay `bg-slate-950/80 backdrop-blur-sm animate-fade-in`, content `animate-modal-in rounded-xl border border-stroke bg-card p-6`.

Modified:
- `web/src/app/globals.css` — add `--accent-fg` (`:root` + `.dark`) and `--color-brand-primary-fg` bridge (~3 lines).
- `components/ui/badge.tsx` — rewrite to variant API + domain mappers; update all existing call sites (grep `from "@/components/ui/badge"`: tasks-tab, task detail, rules, agents views).
- `components/ui/confirm-dialog.tsx` — reimplement on Dialog + Button, **props unchanged**, delete manual focus trap.

## Phase 2 — Dashboard list + Create Project modal (Completed)

- `web/src/app/page.tsx` — CTAs → `Button`; empty state → shared `EmptyState` (extend it with optional `action?: ReactNode`); error banner → semantic danger tokens (`border-danger/20 bg-danger/5 text-danger`); remove stray green glow `shadow-[0_0_15px_rgba(34,197,94,0.2)]` on the blue-brand button (line ~90).
- `components/dashboard/home/project-card.tsx` — delete local `StatusBadge`, use `Badge` + `projectStatusBadge` mapper; skeleton → `Skeleton`; keep `glow-on-hover`.
- `create-project-modal.tsx` + `project-modal/ProjectInfoStep.tsx` + `LinkRepoStep.tsx` — mount on `Dialog` (`dismissable={!isSubmitting}`); form controls → `Input`/`Select`/`Textarea`/`Field`; buttons → `Button`; add 2-dot step indicator.
- `stats-cards.tsx`, `setup-checklist.tsx` — token/spacing pass only.

## Phase 3 — Project detail shell, navigation, header (Completed)

- `web/src/app/projects/[id]/page.tsx` — replace `useState<ProjectView>` with validated `useSearchParams()` read; shortcuts → `router.replace`; error/loading → `Button`/`Skeleton`/semantic tokens. Shell layout kept.
- `components/projects/project-sidebar.tsx` — items become `<Link href={/projects/${id}?view=…}>` with `aria-current`; add home/logo link at top; add `<kbd>1</kbd>`–`<kbd>5</kbd>` hint chips (desktop); keep "Back to Projects" footer.
- `components/projects/ProjectHeader.tsx` restructure:
  - Row 1: linked breadcrumb (`<Link href="/">Projects</Link> / name`); on mobile doubles as back affordance (chevron + "Projects").
  - Row 2: title + copyable id chip (clipboard + sonner toast) + right-aligned Create Task `Button`.
  - Row 3 (`md:hidden`): horizontal scrollable link tab strip replacing the `<select>`; active = `text-brand-primary border-b-2 border-brand-primary`.
  - Removed: 4 count chips, active-now pill, `WorkflowStageStrip` (both move to status summary in Phase 4). Props shrink accordingly.
- **Update `web/e2e/auth-and-dashboard.spec.ts`** (~line 56): `getByRole("button", { name: "Repositories" })` → `getByRole("link", …)`.

## Phase 4 — Tab bodies + settings form (Completed)

- `project-status-summary.tsx` — raw emerald/amber/rose/blue → `text-success`/`text-warning`/`text-danger`/`text-info` (+ `border-*/20 bg-*/5`); absorb `WorkflowStageStrip` + "N active now".
- `tasks-tab.tsx`, `agents-view.tsx`, `repositories-view.tsx`, `rules-view.tsx` — adopt `EmptyState`, `Skeleton`, `Button`, Badge mappers.
- `AddRepositoryForm.tsx`, `rules/AddRuleForm.tsx`, `RepositoryListItem.tsx`, `RuleCard.tsx`, `create-task-panel.tsx` — form controls → primitives (kills remaining `text-slate-950` brand buttons).
- `project-profile.tsx` (settings) — rebuild on `Card`/`Field` primitives; dirty-state tracking vs project snapshot; Save disabled until dirty; secondary "Reset" button; success toast; gate the existing `useEffect` sync to non-dirty state so it can't clobber in-progress edits.
- `tasks/[taskID]/page.tsx`, `TaskActions.tsx`, `task-action.tsx`, `task-pr-review.tsx` — brand-button color fix via `Button` only (full task-detail redesign out of scope).

## Phase 5 — Sweep, docs, deprecation (Completed)

- Gate: Verified and resolved raw `bg-brand-primary` / `text-slate-950` contrast compliance issues in `AddRepositoryForm.tsx`, `RepositoryListItem.tsx`, `agents-view.tsx`, `create-task-panel.tsx`, `task-action.tsx`, `task-pr-review.tsx`, and `projects/[id]/page.tsx`.
- `.design-sync/conventions.md` — document `text-brand-primary-fg` + new primitive set.
- `.design-sync/NOTES.md` — record widened DS scope; note Radix Dialog portal supersedes the old centering workaround; flag `dtsPropsFor`/previews as needing re-sync.
- `docs/design-system/autocodeos/MASTER.md` — prepended deprecation banner pointing at `.design-sync/conventions.md` (don't delete).
- Persist this plan as `docs/plans/PLAN-project-ui-redesign.md` per workspace convention.

## Verification (every phase)

1. `cd web && npm run lint && npm run build` (only scripts; no unit-test runner exists).
2. `npx playwright test` — config self-starts dev server on :3001 with API mocks (`e2e/fixtures/api-mocks.ts`); the login → open project → Repositories flow exercises Phases 2–4 directly. Selector update required in Phase 3.
3. Manual pass via `npm run dev`: both themes, widths 375/768/1280; specifically dialog centering, `?view=` reload persistence, mobile back affordance, settings dirty/reset.
4. After Phase 5, re-run `.design-sync` re-sync steps from NOTES.md to keep the DS export current.

## Critical files

- `web/src/app/globals.css` — the `--accent-fg` token everything keys off
- `web/src/app/projects/[id]/page.tsx` — tab routing, shell, states
- `web/src/components/projects/ProjectHeader.tsx` — header restructure + mobile nav
- `web/src/components/ui/badge.tsx` — primitive style template + v2 rewrite
- `web/src/components/dashboard/home/create-project-modal.tsx` — Dialog + form-primitive exemplar
- `web/e2e/auth-and-dashboard.spec.ts` — regression gate (updated in Phase 3)
