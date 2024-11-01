# Build and test stage
FROM --platform=$BUILDPLATFORM golang:1.22 AS builder
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG SKIP_TESTS=false

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
RUN go mod download

# Copy the entire source code
COPY . .

# Build and test
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go && \
    if [ "$SKIP_TESTS" = "false" ]; then \
        CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go test ./... -v; \
    fi

# Final stage
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
