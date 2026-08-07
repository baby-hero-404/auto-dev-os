import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

// Mirrors the TaskStatus union in web/src/lib/types.ts (TS unions aren't
// runtime-introspectable, so this literal has to be kept adjacent to that
// type by hand — see server/pkg/models/task_status_test.go for the backend
// side of this parity check, which derives its list from source instead of
// a hand-kept literal since Go can parse its own AST).
const FRONTEND_TASK_STATUSES = [
  "todo",
  "context_loading",
  "analyzing",
  "spec_review",
  "coding",
  "reviewing",
  "fixing",
  "testing",
  "pr_ready",
  "human_review",
  "merged",
  "failed",
  "blocked",
];

const FIXTURE_PATH = path.resolve(
  __dirname,
  "../../docs/openspecs/status-driven-agent-workspace/task-statuses.generated.json",
);

// No browser fixture is used here — this is a pure logic check placed under
// e2e/ (rather than web/src/lib/status/__tests__/) because the repo has no
// jest/vitest, only @playwright/test; see docs/implementation/
// status-driven-agent-workspace-notes.md for the reasoning.
test("TaskStatus union matches the backend-generated fixture", () => {
  const fixture: string[] = JSON.parse(fs.readFileSync(FIXTURE_PATH, "utf-8"));

  expect([...FRONTEND_TASK_STATUSES].sort()).toEqual([...fixture].sort());
});
