## Using this design system

This is the UI kit for **auto_code_os**, a Next.js app. It ships as plain
Tailwind v4 utility classes over a semantic token layer — there is no
provider/wrapper component and no CSS-in-JS. Nothing needs to wrap your
markup to get styling: just use the classes below and the components render
correctly on their own.

### Styling idiom: semantic Tailwind utility classes

Never reach for raw Tailwind color scales (`bg-slate-900`, `text-zinc-400`)
or arbitrary hex/oklch values — this DS maps its palette onto semantic class
names. Use these families:

| Purpose | Classes |
| --- | --- |
| Page/surface background | `bg-background`, `bg-surface`, `bg-panel`, `bg-panel-muted` |
| Card/modal background | `bg-card`, `text-card-fg` |
| Text | `text-content` (primary), `text-content-muted` (secondary) |
| Borders | `border-stroke`, `border-stroke-focus` (focus rings) |
| Brand accent | `bg-brand-primary`, `text-brand-primary`, `bg-brand-primary-muted` |
| Status | `text-success` / `bg-success`, `text-warning`, `text-danger`, `text-info` (and the equivalent `bg-*`) |
| Fonts | `font-sans` (body/UI — Inter), `font-mono` (code — JetBrains Mono), `font-heading` (headings — Inter) |
| Radius | `rounded` uses the theme's `--radius` scale automatically (`rounded-md`, `rounded-lg`, `rounded-xl`) — don't hardcode pixel radii |

Every one of these classes resolves against a light/dark CSS-variable pair
(`:root` vs `.dark`) already defined in the shipped stylesheet — components
built with them are dark-mode-correct automatically. Never invent a new
semantic name; if nothing above fits, fall back to a plain Tailwind utility
rather than guessing at a token that doesn't exist.

A few components also use hardcoded Tailwind color-scale utilities for
one-off status semantics that don't map onto the shared tokens (e.g.
`ConfirmDialog`'s danger button: `bg-red-500 hover:bg-red-600`, or `Badge`'s
per-status color map: `bg-emerald-400/10 text-emerald-300`). That's an
existing pattern in the source, not a mistake — follow it only when a status
color truly isn't one of the semantic tokens above.

### Motion

Reusable keyframe classes are already defined in the shipped stylesheet —
use them instead of writing new `@keyframes`: `animate-fade-in` (entrance
fade), `animate-modal-in` (scale+slide-up, use for dialogs/popovers),
`animate-pulse-dot` (status-dot pulse), `animate-completion-pop` (spring pop
for success/completion states).

### Page references

`guidelines/pages.md` has full-screen captures of every real route in the
app (mock data, real layout/composition) — reference-only, not mountable
components. Use them to see how these pieces actually get composed in
context before designing a new screen.

### Where the truth lives

Read `styles.css` (and its `@import`ed `_ds_bundle.css`) for the full token
and utility set before styling anything non-trivial — it's the actual
compiled stylesheet these components render against, tokens included. Each
component's `.prompt.md` documents its own props with real usage examples.

### Example — building a status card with these components

```tsx
import { Badge } from "web/components/ui/badge";
import { EmptyState } from "web/components/ui/empty-state";

function TaskSummaryCard({ task }) {
  return (
    <div className="rounded-lg border border-stroke bg-card p-5">
      <div className="flex items-center justify-between">
        <h3 className="font-heading text-base font-semibold text-content">
          {task.title}
        </h3>
        <Badge value={task.status} />
      </div>
      <p className="mt-2 text-sm text-content-muted">{task.summary}</p>
    </div>
  );
}
```
