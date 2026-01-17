# Build the manager binary
# Build the manager binary
FROM golang:1.24 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# 1. Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# 2. Download dependencies (this layer is cached)
RUN go mod download

# --- CHANGED SECTION STARTS HERE ---
# Instead of "COPY . .", we explicitly copy the folders we need.
# This guarantees that 'internal' is not skipped.
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
# -----------------------------------

# 3. Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
