# 🚀 Auto Code OS

Auto Code OS is an AI-native SDLC platform that orchestrates autonomous agents to analyze tasks, draft specifications, implement backend and frontend code, run verification tests, create Pull Requests, and wait for human merge approval.

---

## 🏗️ System Architecture & Execution Mode

This project is configured as a **hybrid host-container development environment**:
*   **Infrastructure (PostgreSQL Database)** runs containerized via Docker/Docker Compose.
*   **Application Servers (Go API Backend & Next.js Frontend)** run directly on the local host to ensure rapid hot-reloading and lightweight execution.
*   **Agent Sandbox Environments** execute inside isolated Docker containers using the sandbox image built from `docker/Dockerfile.sandbox`.

---

## 🛠️ Repository Layout

```text
/
├── server/                # Go backend monorepo (API, workflow, sandbox runtime, database layer)
│   ├── cmd/               # Entry points (CLI, API server, DB migrations)
│   ├── internal/          # Domain services (gitops, sandbox agent, models, orchestrator)
│   └── pkg/               # Common packages (configurations, unified LLM gateway)
├── web/                   # Next.js frontend web app dashboard
│   ├── src/app/           # Next.js App Router pages
│   ├── src/components/    # Reusable React components
│   └── src/lib/           # Frontend APIs and type contracts
├── docs/                  # Documentation, architectural definitions, and feature plans
├── docker/                # Containment templates (Sandbox Dockerfile)
├── docker-compose.yml     # Local database and infrastructure compose specification
└── Makefile               # Reorganized workflow automation commands
```

---

## 🚦 Developer Onboarding & Quick Start

Follow these 4 simple steps to set up your environment and spin up the application:

### 1. Install Prerequisites
Ensure you have the following installed on your host machine:
*   **Go** 1.26+
*   **Node.js** 20+ (with `npm`)
*   **Docker & Docker Compose**

### 2. Initialize the Project
Run the initialization target to automatically create your local `.env` configuration file and install the Next.js frontend dependencies:
```bash
make init
```

### 3. Build the Agent Docker Sandbox
Before launching tasks, you must build the sandbox Docker image that is used by the autonomous agent to securely clone and verify code:
```bash
make sandbox-build
```
> [!IMPORTANT]
> The sandbox container isolates the agent's code execution, running tests, and compiling code. Without building this image, agents running tasks in `docker` mode will fail to start.

### 4. Configure Environment Secrets (Optional)
If you wish to configure LLM keys via environment variables (as a fallback), open the `.env` file and set your credentials:
```env
LLM_PROVIDER=openai  # Supported: openai, anthropic, gemini, gateway
OPENAI_API_KEY=sk-your-openai-api-key-here
# Optional: ANTHROPIC_API_KEY / GEMINI_API_KEY
```
> [!NOTE]
> Environment API keys are optional fallbacks. You can also configure all provider credentials dynamically directly through the Web UI Dashboard once the application is running.

---

## 💻 Running the Application

### Start the Full Stack (Database in Docker, Apps on Host)
To clear any port conflicts, spin up PostgreSQL, run all migrations, and launch the Go backend and Next.js web servers:
```bash
make dev
```

*   **Go API Server URL:** [http://localhost:8080](http://localhost:8080)
*   **Next.js Web UI URL:** [http://localhost:3000](http://localhost:3000)

### Segmented Development Execution
If you prefer running the backend or frontend in isolation:
*   **Run Backend Only (DB + Migrations + API):**
    ```bash
    make dev-be
    ```
*   **Run Frontend Only (Next.js server):**
    ```bash
    make dev-fe
    ```

---

## 🧪 Testing & Code Quality

Verify your changes using the following targets:

*   **Run all tests (Go Backend + Playwright E2E):**
    ```bash
    make test
    ```
*   **Run backend unit & integration tests only:**
    ```bash
    make test-be
    ```
*   **Run frontend Playwright tests only:**
    ```bash
    make test-fe
    ```
*   **Lint code & check formatting:**
    ```bash
    make lint
    make fmt
    ```

---

## 📚 Development Invariants & Best Practices

### 🔄 Task & Workflow Lifecycles
*   **Durable Orchestration:** Tasks follow a strict DAG path: `todo -> context_loading -> analyzing -> spec_review -> coding -> reviewing -> fixing -> testing -> pr_ready -> human_review -> merged`.
*   **PR Creation !== Task Completion:** A task is only complete once a human has explicitly reviewed and approved/merged the generated Pull Request.
*   **Branching Complexity:**
    *   *Easy Tasks:* run straight through `context_load -> analyze -> code_backend -> test -> pr`.
    *   *Medium/Hard Tasks:* enforce `context_load -> analyze -> plan -> code_backend/code_frontend -> merge -> review -> fix -> test -> pr`.

### 🛡️ Security, Sandbox & Secrets
*   **Keys and Tokens:** Never commit API keys, GitHub developer tokens, or database credentials. Use the host `.env` file or environment variables.
*   **Sandbox Isolation:** Under no circumstances should agent commands execute directly on the host machine. All code operations must go through the sandbox driver.

---

## 📝 Features & Architecture References
Refer to the following feature logs under `docs/features/` to understand component specifications:
*   [5.1: Unified AI Gateway](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.1-unified-ai-gateway.md)
*   [5.2a: Rule System](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.2a-rule-system.md)
*   [5.2b: Skill System](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.2b-skill-system.md)
*   [5.3: Agent System](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.3-agent-system.md)
*   [5.4: Git Integration](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.4-git-integration.md)
*   [5.5: Project System](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.5-project-system.md)
*   [5.6: Task System](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.6-task-system.md)
*   [5.7: Workflow Engine](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.7-workflow-engine.md)
*   [5.8: PR Human Review](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.8-pr-human-review.md)
*   [5.9: Dashboard Analytics](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.9-dashboard-analytics.md)
*   [5.10: Multi-Channel Interaction](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.10-multi-channel-interaction.md)
*   [5.12: Patch Engine Abstraction](file:///home/ubuntu/my_projects/auto_code_os/docs/features/5.12-patch-engine-abstraction.md)
