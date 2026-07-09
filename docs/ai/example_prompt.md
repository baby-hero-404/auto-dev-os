# Example Prompt Structure

This document details the modular rendered prompt structure produced by the new pipeline assembler.

```markdown
# System Prompt
========
[Base Prompt] (Immutable)
You are an AI Agent...

[Role Prompt] (Immutable)
You are a Backend Developer... Goal: ...

[Step Prompt]
You are executing code_backend. Your goal is...

[JIT Skills] (Dynamic allowed tools registered from these skills)
(golang-best-practices.md)
(clean-code.md)

[Rules] (Layered)
- [Global - Strict] (Immutable) No raw SQL...
- [Role Constraint - Strict] (Immutable) Do not modify non-backend files.
- [Project - Advisory] Use standard logging helper.

[Context] (Sliced)
### Architecture Summary
...
### Semantic Snippets
...

[Task]
### Requirement
...
```
