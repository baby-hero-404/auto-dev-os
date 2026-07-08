import type { Rule } from "@/lib/types";

export const DEFAULT_GLOBAL_RULES: Array<{ content: string; enforcement: Rule["enforcement"] }> = [
  {
    content: "Follow clean code principles: self-documenting code, meaningful variable names, small focused functions.",
    enforcement: "strict",
  },
  {
    content: "All code changes must include tests. No PR may be merged without passing CI.",
    enforcement: "strict",
  },
  {
    content: "Use conventional commit messages: feat:, fix:, docs:, refactor:, test:, chore:.",
    enforcement: "advisory",
  },
  {
    content: "Security first: never log secrets, validate all inputs, use parameterized queries.",
    enforcement: "strict",
  },
  {
    content: "Document architectural decisions in ADRs. Update ARCHITECTURE.md when adding new packages or changing data flow.",
    enforcement: "advisory",
  },
  {
    content: "Strictly enforce the Socratic Gate (Definition of Ready): before starting implementation on any Medium/Hard tasks, ask the user at least 3 strategic questions to clarify specifications and boundary conditions. Do not start coding until requirements are explicitly confirmed.",
    enforcement: "strict",
  },
  {
    content: "Ensure all code edits are surgical and targeted. Modify only the necessary parts of the codebase, preserving surrounding code style, docstrings, and comments.",
    enforcement: "strict",
  },
  {
    content: "Practice Progressive Discovery and JIT Knowledge: read specific line ranges rather than loading entire files. Dynamically load/unload task-specific skills and remove them from context once the subtask is complete to avoid context window overflow.",
    enforcement: "strict",
  },
  {
    content: "Always perform self-checks and verify your implementation by running local tests and linting before marking a task as complete.",
    enforcement: "strict",
  },
];
