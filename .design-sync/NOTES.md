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

## Page references (`guidelines/pages/`)

User asked to widen the sync beyond the 4 atomic components to include
**whole real pages**, explicitly as non-interactive reference material (not
mountable DS components) — "what pages actually look like" when the kit's
pieces are composed in context. Since none of the app's routes are
themselves reusable/composable pieces (they're thin `DashboardLayout`
wrappers around data-bound feature components, per `web/src/app/agents/
page.tsx` etc.), these ship as flat PNG screenshots + an index, not as
`components/` cards.

**Capture method**: Playwright, full-page screenshots at default viewport,
against the real app driven by the repo's own e2e mock fixtures
(`web/e2e/fixtures/api-mocks.ts`: `installApiMocks` + `seedSession` +
`createMockState`), plus a catch-all `**/api/v1/**` route registered
*before* `installApiMocks`'s specific routes (Playwright: last-registered
route wins on overlap) so any endpoint the fixtures don't cover returns an
empty 200 instead of hanging/erroring. 15 routes captured; excluded the
legacy `/tasks/[id]` redirect stub (no rendered UI of its own — pure
`useEffect` → `router.push`).

The maintenance script lives at `.design-sync/capture-pages.spec.ts` (not
part of the real e2e suite — outside `web/e2e/` so Playwright's `testDir`
never picks it up). To regenerate:

```
cp .design-sync/capture-pages.spec.ts web/e2e/_page-capture.spec.ts
cd web && SKIP_WEBSERVER=1 PLAYWRIGHT_PORT=<port-of-a-running-dev-server> \
  npx playwright test e2e/_page-capture.spec.ts --reporter=list
rm e2e/_page-capture.spec.ts
```

Output lands in `.design-sync/pages/*.png` (the script's `OUT_DIR` is
`../.design-sync/pages`, relative to `web/`).

**Each page also has a `.html` card wrapper** (`.design-sync/pages/<Name>.html`,
generated once by a one-off script — regenerate similarly if pages are
added/removed): a minimal `<!-- @dsCard group="Pages" viewport="WxH" -->`
page that just `<img src="<Name>.png">`s the screenshot at half-scale. This
is what makes them show up as browsable cards in the claude.ai/design app's
picker — files under `guidelines/` with no `@dsCard` marker are uploaded and
readable via `read_file`/the design agent, but **invisible in the app's own
UI**, which only lists `@dsCard`-marked `.html` files (learned this the hard
way — first pass shipped only the `.png`+`.md`, user reported not seeing
them in the app at all).

**Re-sync risk — none of this survives `package-build.mjs`**: every build
runs `rmSync(OUT, { recursive: true, force: true })` on the whole
`ds-bundle/` output dir, and the only thing that auto-repopulates
`guidelines/` is `cfg.guidelinesGlob` matching `.md`/`.mdx` files (images
and other `.html` are explicitly skipped — see `.ds-sync/lib/docs.mjs`'s "is
not .md/.mdx — skipped" check). So after any future full resync, re-copy
manually before re-uploading:

```
mkdir -p ds-bundle/guidelines/pages
cp .design-sync/pages/*.png .design-sync/pages/*.html ds-bundle/guidelines/pages/
cp .design-sync/pages/pages.md ds-bundle/guidelines/pages.md
```

The pointer to `guidelines/pages.md` lives in `.design-sync/conventions.md`
(the `readmeHeader`), so it regenerates into `README.md` automatically on
rebuild — only the `guidelines/pages/` images and `guidelines/pages.md`
itself need the manual re-copy. `_ds_sync.json`'s `auxSha` covers
`guidelines/` + `README.md`, so after re-copying, recompute it before
upload: `node --input-type=module -e "import { auxShaFor } from
'./.ds-sync/lib/sync-hashes.mjs'; console.log(auxShaFor('./ds-bundle'))"`
and patch the `auxSha` field in `ds-bundle/_ds_sync.json` to match — a full
`resync.mjs` run recomputes this for you automatically; only a
build-mjs-only run needs the manual patch.

Uploaded via a scoped `finalize_plan`/`write_files` (writes:
`guidelines/pages/**`, `guidelines/pages.md`, `README.md`,
`_ds_sync.json`, `_ds_needs_recompile`) rather than the full atomic-path
plan, since this was an incremental addition on top of an already-verified
sync. **Always include `_ds_needs_recompile`** (write it first as a fence,
then again last after the real files) — it's what makes the app rebuild
`_ds_manifest.json` and pick up new `@dsCard` cards; without re-writing it,
new cards silently don't appear even though the files are uploaded fine.
The manifest itself only recompiles when the project is opened/refreshed in
the browser, not immediately on upload — don't be alarmed if `read_file`
on `_ds_manifest.json` right after upload still shows the old card list.
