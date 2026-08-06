FROM autocode-base:latest

USER root
# Match the Go toolchain version used elsewhere in this repo's own build
# (see docker/Dockerfile.sandbox's mcpbuilder stage / go.mod).
ARG GO_VERSION=1.24.0
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz" -o /tmp/go.tar.gz \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz
# Enable GOTOOLCHAIN=auto so Go automatically fetches matching Go versions from go.mod
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOTOOLCHAIN=auto
ENV GOPATH=/home/agent/go
ENV PATH="${GOPATH}/bin:${PATH}"
USER agent
