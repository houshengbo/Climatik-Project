# Build stage
FROM --platform=$BUILDPLATFORM nvidia/cuda:12.0.0-devel-ubuntu22.04 AS builder
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG SKIP_TESTS=true

# Install Go
RUN apt-get update && apt-get install -y \
    wget \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.21
RUN wget https://go.dev/dl/go1.21.4.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.21.4.linux-amd64.tar.gz \
    && rm go1.21.4.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Cache deps before building
RUN go mod download

# Copy the source code
COPY . .

# Build with CUDA environment
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Final stage
FROM --platform=$TARGETPLATFORM nvidia/cuda:12.0.0-base-ubuntu22.04
WORKDIR /

# Copy binary
COPY --from=builder /workspace/manager .

# Copy configuration files and manifests
COPY --from=builder /workspace/config/crd/bases /config/crd/bases
COPY --from=builder /workspace/config/gpu_frequencies.json /config/gpu_frequencies.json

# Create necessary directories with proper permissions
RUN mkdir -p /config && chown -R 65532:65532 /config

USER 65532:65532

LABEL maintainer="Chen.Wang1@ibm.com"
LABEL version="1.0"

EXPOSE 8080
EXPOSE 8081

ENTRYPOINT ["/manager"]