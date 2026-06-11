# syntax=docker/dockerfile:1.7

# Build stage: cross-compile on the buildx host's native arch, then COPY
# the right per-arch binary into the final stage. Avoids QEMU emulation
# (much faster; same result for a static Go binary).
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Cache module download across builds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# Build a fully static, stripped binary. CGO_ENABLED=0 → no glibc dep, runs
# on scratch/distroless. -trimpath strips local paths from the binary.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/captcha-solver-mcp \
      ./cmd/captcha-solver-mcp

# Final stage: distroless static + nonroot. Matches the k8s securityContext
# (runAsNonRoot, readOnlyRootFilesystem, drop ALL caps, seccompRuntimeDefault).
# No shell — the binary handles SIGTERM via signal.NotifyContext in main.go.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/captcha-solver-mcp /captcha-solver-mcp

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/captcha-solver-mcp"]
