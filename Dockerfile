# Frontend build stage
FROM node:22-alpine AS build-frontend
WORKDIR /app/frontend

RUN corepack enable && corepack prepare pnpm@9 --activate

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# Go build stage
FROM golang:1.26-alpine AS build-go
ENV CGO_ENABLED=0
ARG BUILD_VERSION

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . /app
COPY --from=build-frontend /app/frontend/dist /app/frontend/dist

RUN --mount=type=cache,target=/root/.cache/go-mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-w -s" -o shepherd .

# Final stage
FROM gcr.io/distroless/base:nonroot
USER nonroot
COPY --from=build-go /app/shepherd /shepherd
ENTRYPOINT ["/shepherd"]
