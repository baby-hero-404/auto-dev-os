# Coding Patterns — Auto Code OS

## Handler Pattern
1. Parse & validate request payload (JSON bind / query parameters).
2. Call business logic services (avoid database calls or complex orchestration in handlers).
3. Return structured JSON responses with unified error structures.
4. Keep router definitions separate from handlers.

## Service Pattern
1. Handle business logic, authorization validation, and Orchestrator interaction.
2. Return domain-specific errors (not HTTP status codes).
3. Coordinate multiple repository operations inside transactions when necessary.

## Repository Pattern
1. **GORM Database Access**: All database operations must use the GORM (`gorm.io/gorm`) architecture. Do not write raw connection or manual SQL pool logic unless it's pgvector-specific vector syntax that GORM doesn't support natively.
2. Limit repositories strictly to DB queries and table inserts/updates/deletes.
3. Return domain models defined under `server/pkg/models/`.

## Sandboxed Isolation & HITL
1. All agent-triggered execution tasks must run inside isolated Docker containers (via Sandbox client).
2. Human-in-the-Loop (HITL) approval is strictly required before committing, merging, or modifying deployment configurations for Medium or Hard tasks.

## Testing Conventions
1. Author comprehensive unit/integration tests for every added feature or bug fix.
2. Keep test files alongside code in `*_test.go`.
3. Use mock interfaces rather than actual database/Docker clients for unit tests.
