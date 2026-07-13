import { Markdown } from "../../web/src/components/ui/markdown";

const SPEC = `## Proposal

Add a \`retry\` step to the gateway so a failed provider call falls back to the next credential in the pool.

- Detect \`429\`/\`5xx\` responses and rotate credentials
- Log each attempt with the provider name
- Give up after 3 attempts and surface the last error

> This mirrors the retry policy already used by the orchestrator's tool loop.

| Attempt | Credential | Result |
| --- | --- | --- |
| 1 | primary | 429 |
| 2 | backup-a | 200 |

Call \`RotateCredential(ctx)\` between attempts.
`;

export function TaskSpec() {
  return <Markdown content={SPEC} />;
}

export function InlineNote() {
  return <Markdown content="Uses the `gateway.go` retry loop — see the `RotateCredential` helper for details." />;
}
