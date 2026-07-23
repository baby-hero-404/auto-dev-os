# Implementation notes: RepoMap Mention Boost

## Baseline already in place

`ExtractMentionedIdents`, `mentionBoostFactor = 10.0`, and the identifier-mention wiring in
`ranking.go`'s `CalculatePageRank` (REQ-001, REQ-002, REQ-004) already existed before this pass —
they were built as part of an earlier, undocumented change. `internal/prompts/builder.go:1019`
already wired `task.Title+"\n"+task.Description` into `provider.TaskDescriptionKey`, consumed by
`provider.GetRepoMap` at `provider.go:319-320` and passed straight into `CalculatePageRank`. This
pass only needed to add the missing piece: **REQ-003, path-mention = active-file boost**.

## REQ-003: path mention treated as active file (×50)

Added `ExtractMentionedPaths` (`mentions.go`) — a regex (`[A-Za-z0-9_\-./]*[A-Za-z0-9_\-]+\.[A-Za-z0-9]+`)
pulling path-shaped tokens (bare filenames like `policy_engine.go`, or slash paths like
`server/internal/policy_engine.go`) out of free text. Filtering against files that actually exist
in the repo graph happens at the ranking layer (`mentionedFileNodeIDs` in `ranking.go`), matching
by exact path or `/`-suffix, mirroring how `mentionedNodeIDs` filters identifier mentions against
`defsByFile`.

`CalculatePageRank` now merges `activeFiles ∪ mentionedFileNodeIDs(taskDescription)` into a single
`fileBoostIDs` set, used for:
1. Personalization-vector seeding (previously only `activeFiles`) — kept symmetric per design.md's
   note to "seed mentioned files like active files if the active-file mechanism does".
2. The inbound-edge-weight boost inside `boostedWeight` (50x, taking priority over the 10x ident
   boost — mutually exclusive per design.md, no stacking to 500x).
3. The final post-hoc 50x multiplier pass (previously only iterated `activeFiles`).

An empty `taskDescription` makes `mentionedFileNodeIDs` return an empty set, so `fileBoostIDs`
degrades to exactly `activeFiles` — REQ-004's byte-identical-with-no-mention guarantee holds.

## Design deviations

- **No `ExtractMentions(text, known, knownFiles) Mentions` unified type/signature** as design.md
  sketched. Kept the pre-existing pattern of two separate extractor functions
  (`ExtractMentionedIdents`, `ExtractMentionedPaths`) that return raw candidate sets, with
  known-definition/known-file filtering done at the ranking layer instead of inside the
  extractors. Net filtering behavior is the same; this avoids reshaping the already-working
  ident-mention code path.
- **No `WithMentions(text)` variadic builder option** on `BuildRepoMap`. The existing
  `taskDescription string` parameter on `CalculatePageRank`, fed via the
  `provider.TaskDescriptionKey` context value, already satisfies REQ-M01 (optional, backward
  compatible, no-op on empty string) without a public API change.
- Backtick-delimited mentions (e.g. `` `CreateGitCheckpoint` ``) are not special-cased separately —
  backticks are non-identifier/non-path runes, so they act as natural token boundaries for both
  extractors and the content extracts correctly without dedicated backtick-parsing logic.
