# Go build stage
FROM golang:1.25-alpine AS build-go
ENV CGO_ENABLED=0
ARG BUILD_VERSION

# Install git for go mod operations
RUN apk add --no-cache git

WORKDIR /app

# Set up Go module cache directory
ENV GOCACHE=/root/.cache/go-build
ENV GOMODCACHE=/root/.cache/go-mod

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./

# Download dependencies with cache mount
RUN --mount=type=cache,target=/root/.cache/go-mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# Copy source code
COPY . /app

# Build the application with cache mounts
RUN --mount=type=cache,target=/root/.cache/go-mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-w -s" -o shepherd

# Final stage
FROM gcr.io/distroless/base:nonroot
USER nonroot
COPY --from=build-go /app/shepherd /shepherd

ENTRYPOINT ["/shepherd"]
