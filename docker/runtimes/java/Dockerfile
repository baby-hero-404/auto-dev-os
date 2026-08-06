FROM autocode-base:latest

USER root
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && apt-get install -y --no-install-recommends \
        default-jdk \
        maven \
    && rm -rf /var/lib/apt/lists/*
USER agent
