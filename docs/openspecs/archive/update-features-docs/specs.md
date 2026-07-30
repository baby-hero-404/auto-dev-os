# Expected Behavior: Feature Docs Audit & Sync

## Scenario: Code-to-Doc Parity Check
**When:**
- The audit process runs.
**Then:**
- It must map each `server/internal/` package (e.g., `gateway/`, `governance/`, `policy/`, `tool/`) to its corresponding feature document.
- If a package has no document, a new document must be created.
- If a document has no corresponding code, it must be removed or marked as `⚪ Deferred`/`Archived`.

## Scenario: Status Alignment
**When:**
- Evaluating a feature document's `Status`.
**Then:**
- It must be `🟢 Implemented` if the feature is actively running in production code.
- It must be `🟡 In Progress` if only the baseline is implemented.
- It must be `🔵 Proposed` if the code does not exist yet.

## Rules
- **No Hallucinations:** Every documented feature must include a `sources:` frontmatter block pointing to the exact files (`server/internal/...`) that implement it.
- **Single Source of Truth:** Do not duplicate policies across files. Use cross-references (`§NN`).
- **Language Convention:** Vietnamese for body prose, English for technical terms, file paths, and headers.

## Constraints
- The `docs/features/README.md` must be regenerated to reflect the final list of documents and their freshness dates.
