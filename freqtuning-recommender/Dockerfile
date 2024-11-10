# Build stage
FROM --platform=$BUILDPLATFORM golang:1.22.0 AS builder
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

# Set up proper Go module path
WORKDIR /workspace/src/github.com/Climatik-Project/Climatik-Project

# Copy the entire project directory
COPY ../.. .

# Change to freqtuning-recommender directory
WORKDIR /workspace/src/github.com/Climatik-Project/Climatik-Project/freqtuning-recommender

# Cache deps before building
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Final stage
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static:nonroot
WORKDIR /

# Copy binary
COPY --from=builder /workspace/src/github.com/Climatik-Project/Climatik-Project/freqtuning-recommender/manager .

# Copy power profile data
COPY --from=builder /workspace/src/github.com/Climatik-Project/Climatik-Project/freqtuning-recommender/config/power_profiles /config/power_profiles

USER 65532:65532

LABEL maintainer="Chen.Wang1@ibm.com"
LABEL version="1.0"

EXPOSE 8080
EXPOSE 8081

ENTRYPOINT ["/manager"] 