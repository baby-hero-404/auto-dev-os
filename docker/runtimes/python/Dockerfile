FROM autocode-base:latest

USER root
RUN mkdir -p /etc/pip && echo "[global]\nbreak-system-packages = true" > /etc/pip.conf
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && apt-get install -y --no-install-recommends \
        python3 \
        python3-pip \
        python3-pytest \
    && rm -rf /var/lib/apt/lists/*

# Best Practice: Install uv for ultra-fast Python dependency & version management
RUN pip install --no-cache-dir uv

USER agent
