# Tasks: RepoMap Mention Boost

- [x] 1.1 `repomap/mentions.go`: `ExtractMentionedIdents` (pre-existing) + new `ExtractMentionedPaths` for backticks/paths, table tests incl. song ngữ — filtering against known defs/files done at the ranking layer (`mentionedNodeIDs`/`mentionedFileNodeIDs`), not inside the extractors themselves (deviation from design.md's single `ExtractMentions(text, known, knownFiles)` signature, but same net filtering behavior)
- [x] 1.2 `ranking.go`: boost ×10 idents / ×50 files (mutually-exclusive, file wins) + personalization vector and final multiplier both seeded from `activeFiles ∪ mentionedFileNodeIDs` for symmetry with active-files
- [x] 1.3 Wiring already existed pre-spec: `CalculatePageRank(activeFiles, taskDescription)` takes task text directly (not a `WithMentions` builder option as design.md sketched); `internal/prompts/builder.go:1019` passes `task.Title+"\n"+task.Description` via `provider.TaskDescriptionKey` context value, consumed in `provider.go:319-320`. Empty description is a no-op, satisfying REQ-M01 without a signature change.
- [x] 1.4 Rank comparison tests (ident-mention pre-existing; new path-mention test asserting ×50 > ×10 and > baseline) + no-mention snapshot test (pre-existing, still passing) (REQ-002/003/004)
- [x] 1.5 Update specs.md status

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
