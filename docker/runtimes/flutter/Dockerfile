# flutter runtime image: new territory not covered by today's monolith
# sandbox image at all. Installs the Flutter SDK's stable channel via git
# clone (Flutter's own documented install method, no package-manager
# release), following the same "curl a vendor installer script as root into
# /usr/local, then hand off to the non-root agent user" pattern already used
# for antigravity in docker/runtimes/base/Dockerfile — the SDK itself is
# stateless/shared, only its per-project pub cache (mounted from the host
# via the flutter manifest's cache: list) needs to be writable by agent.
FROM autocode-base:latest

USER root
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && apt-get install -y --no-install-recommends \
        unzip \
        xz-utils \
        libglu1-mesa \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 --branch stable https://github.com/flutter/flutter.git /usr/local/flutter \
    && chown -R agent:agent /usr/local/flutter
ENV PATH="/usr/local/flutter/bin:${PATH}"

USER agent
# Pre-fetch Dart/Flutter tool artifacts and accept Android SDK licenses at
# build time (not first-request time) so a task's first `flutter doctor`/
# `flutter pub get` inside the container isn't the thing paying for this.
RUN flutter precache \
    && yes | flutter doctor --android-licenses || true
