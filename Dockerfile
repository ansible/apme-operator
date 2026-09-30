# Build the manager binary.
# Release and docker-buildx set TARGETARCH per platform (linux/amd64 + linux/arm64).
# A bare `docker build` / `make docker-build` without PLATFORM uses the host arch only —
# an amd64-only image will CrashLoop on ARM nodes (exec format error).
FROM registry.access.redhat.com/ubi9/go-toolset:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

# go-toolset runs as UID 1001; GOPATH/GOCACHE must stay writable for
# OpenShift builds that assign an arbitrary UID.
ENV CGO_ENABLED=0 \
    GOCACHE=/tmp/gocache \
    GOPATH=/tmp/gopath

WORKDIR /opt/app-root/src
COPY --chown=1001:0 go.mod go.mod
COPY --chown=1001:0 go.sum go.sum
RUN go mod download

COPY --chown=1001:0 cmd/main.go cmd/main.go
COPY --chown=1001:0 api/ api/
COPY --chown=1001:0 internal/ internal/

# GOARCH has no default so the binary matches the host (or build.openshift.io) arch
# when TARGETARCH is unset; buildx injects TARGETARCH for each platform.
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# UBI Minimal runtime for OpenShift (restricted-v2 / arbitrary UID).
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
WORKDIR /
COPY --from=builder /opt/app-root/src/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
