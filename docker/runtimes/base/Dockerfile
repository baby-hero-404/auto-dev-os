# autocode-base: shared foundation for every per-runtime sandbox image
# (docker/runtimes/{node,python,java,go,flutter}/Dockerfile). This is
# docker/Dockerfile.sandbox minus every per-language toolchain install —
# those move into the runtime-specific Dockerfiles that FROM this image, so
# a task detected as e.g. "python" never pulls/carries node+JDK+go it will
# never use. mcp-context, the CLI agents (claude/codex), and the "agent"
# non-root user are shared by every runtime image, so they stay here.
FROM golang:1.26-bookworm AS mcpbuilder
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=1 go build -o /usr/local/bin/mcp-context ./cmd/mcp-context

FROM debian:bookworm-slim

# Allow host to pass a custom UID/GID to avoid permission issues when mounting volumes
ARG USER_UID=1000
ARG USER_GID=1000

RUN rm -f /etc/apt/apt.conf.d/docker-clean; echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' > /etc/apt/apt.conf.d/keep-cache
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && apt-get install -y --no-install-recommends \
        bash \
        ca-certificates \
        curl \
        git \
        make \
        patch \
    && rm -rf /var/lib/apt/lists/*

# CLI coding agents require Node 22+; installed here (not per-runtime) since
# every runtime image needs the agent CLIs regardless of the project's own
# language — a Python project's sandbox still runs `claude`/`codex` itself
# via Node, it just doesn't need Python-the-language's own toolchain
# duplicated per image.
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

RUN --mount=type=cache,target=/root/.npm \
    npm install -g @anthropic-ai/claude-code @openai/codex \
    && npm cache clean --force

COPY --from=mcpbuilder /usr/local/bin/mcp-context /usr/local/bin/mcp-context

RUN groupadd --gid ${USER_GID} agent \
    && useradd --uid ${USER_UID} --gid ${USER_GID} --create-home --shell /bin/bash agent

WORKDIR /workspace

USER root
ENV PATH="/usr/local/bin:${PATH}"
# See docker/Dockerfile.sandbox for why antigravity is installed as root
# then relocated to /usr/local/bin rather than left under /root/.local/bin.
RUN bash -c 'set -o pipefail; HOME=/root curl -fsSL https://antigravity.google/cli/install.sh | bash && mv /root/.local/bin/agy /usr/local/bin/agy && rm -rf /root/.local'

USER agent
CMD ["bash", "-c", "pwd && ls -la"]
