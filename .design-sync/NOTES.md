# design-sync notes — auto_code_os / web

## Repo shape

`web/` is a private Next.js 16 app (`private: true`, no `main`/`module`/`exports` in
`package.json`) — not a publishable component library. Only 4 components under
`web/src/components/ui/` were judged reusable-enough to sync: `Badge`,
`ConfirmDialog`, `EmptyState`, `Markdown`. Everything else under
`web/src/components/**` (dashboard/, projects/, settings/, auth/) is
page-specific, wired to app data/hooks/stores — not design-system primitives.
User confirmed this scope explicitly (declined widening it).

## Build setup (synth-entry mode)

No `dist/` exists, so this runs in the converter's synth-entry fallback:

- `--entry ./web/src/components/ui/_entry.mjs` — this file **never exists on
  disk**. It's a deliberate placeholder so `package-build.mjs`'s PKG_DIR walk
  climbs from `web/src/components/ui/` up to `web/package.json` (the first
  ancestor with a `name` field), landing `PKG_DIR` on `web/`. The path is
  never read, only walked.
- `cfg.srcDir = "src/components/ui"` scopes `srcRoot` to just that folder —
  without it, srcRoot defaults to `web/src` and synth-entry would
  `export *` from every `.tsx` under the whole app (~56 files, including
  Next.js pages/layouts), which would almost certainly break the esbuild
  bundle.
- `cfg.componentSrcMap` pins all 4 components explicitly (belt-and-suspenders
  with `srcDir` scoping already limiting discovery to those 4 files).
- `--node-modules ./web/node_modules` — has react/react-dom/lucide-react/
  react-markdown/remark-gfm, everything these 4 components import.

## CSS

Tailwind v4 (`@import "tailwindcss"` + `@theme inline` in
`web/src/app/globals.css`) — `cfg.cssEntry` requires an already-**compiled**
stylesheet (it's appended verbatim, never processed), so the raw globals.css
source won't work. Fix: ran `npm run build` in `web/` (Next/Turbopack), copied
the resulting `web/.next/static/chunks/<hash>.css` to
`web/.ds-sync-cache/compiled.css` (gitignored — regenerate on re-sync if
tokens/utility classes changed), pointed `cfg.cssEntry` there (must stay
inside `web/` — cssEntry is bounded to PKG_DIR).

**Re-sync risk**: `.ds-sync-cache/compiled.css` is a snapshot. If
`globals.css` tokens change, or new Tailwind utility classes get added to
these 4 components, re-run `npm run build` in `web/` and re-copy before
re-running the converter — the stale CSS won't error, it'll just silently
ship outdated/missing utility classes.

## `.d.ts` prop extraction

Auto-extraction from source fell back to `{[key: string]: unknown}` for all 4
(installing `typescript` into `.ds-sync/node_modules` didn't fix it — the
`ts-morph` project apparently doesn't resolve enough context from these
standalone files). Hand-wrote real prop shapes via `cfg.dtsPropsFor` for all
4, transcribed directly from each component's own prop destructuring /
interface in source. **Re-sync risk**: if these components' props change,
`dtsPropsFor` will silently go stale (it's an override, not extracted) —
diff against source when re-syncing.

## Known render warns

None outstanding — render check is clean (4/4, no `bad`/`thin`/
`variantsIdentical`) after fixes below.

## `ConfirmDialog` — fixed-position + transformed ancestor bug

`ConfirmDialog` renders `position: fixed; inset: 0` for its backdrop+dialog.
The tool's card wrapper (`.ds-single` / `.ds-cell` in `lib/emit.mjs`'s static
HTML template) sets `transform: translateZ(0)` on the story wrapper, which
per the CSS spec makes that wrapper the **containing block** for descendant
`fixed`-position elements — so the dialog centers against the wrapper's
(near-zero, auto) height instead of the real viewport, rendering pinned to
the top and clipped. Confirmed by toggling the transform off with
`page.addStyleTag` in a scratch script — removing it fixed the centering
completely.

Since `lib/emit.mjs` is off-limits to fork (it's the app-contract surface),
the fix lives in the **authored preview**: `.design-sync/previews/
ConfirmDialog.tsx` wraps each story in `ReactDOM.createPortal(children,
document.body)`, which moves the dialog's actual DOM out of the transformed
ancestor's subtree entirely — `document.body` has no transform, so `fixed`
resolves against the real viewport again. This matches how the component
truly renders in the app (never inside a transformed ancestor there), so the
portal isn't a fudge — it's the accurate representation.

`cfg.overrides.ConfirmDialog = {"cardMode": "single", "viewport": "560x640"}`
is also set (overlay components need `cardMode: single` per the skill's
standard guidance) — this is still needed for the DS pane's card sizing even
with the portal fix.

**Re-sync risk / watch for**: any FUTURE overlay/portal-style component added
to this DS will hit the identical bug. The fix pattern (portal to
`document.body` inside the authored preview) is the template to reuse — grep
this file for "createPortal" for the reference implementation.

## Playwright version

Repo's own `web/node_modules/playwright-core` pins chromium build **1228**,
which was already cached locally (`~/.cache/ms-playwright/chromium-1228`).
Installed `playwright@1.61.1` + `playwright-core@1.61.1` (matching) into
`.ds-sync/node_modules` for the render check — didn't need a fresh chromium
download. Also installed `typescript` and `@types/react` there.

## Preview scope

All 4 components were authored + graded `good` (user chose "author all 4"
given the tiny component count — cheap here, wouldn't be the default choice
at normal DS scale).
