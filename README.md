# Shepherd

A GitHub App webhook handler written in Go that receives and processes GitHub events with signature verification and structured logging.

## Features

- ✅ GitHub webhook event handling (Pull Requests, Releases, Push)
- ✅ HMAC-SHA256 signature verification
- ✅ Clean Architecture (Domain/UseCase/Controller/CLI layers)
- ✅ Structured logging with `slog`
- ✅ Graceful shutdown
- ✅ Health check endpoint
- ✅ Environment variable and CLI flag configuration

## Installation

```bash
go install github.com/m-mizutani/shepherd@latest
```

Or build from source:

```bash
git clone https://github.com/m-mizutani/shepherd.git
cd shepherd
go build -o shepherd
```

## Usage

### Start the server

```bash
# Using environment variables
export SHEPHERD_GITHUB_WEBHOOK_SECRET="your-webhook-secret"
./shepherd serve

# Using CLI flags
./shepherd serve --github-webhook-secret "your-webhook-secret" --addr "localhost:8080"
```

### Configuration

| Environment Variable | CLI Flag | Description | Default |
|---------------------|----------|-------------|---------|
| `SHEPHERD_ADDR` | `--addr` | Server address | `localhost:8080` |
| `SHEPHERD_GITHUB_WEBHOOK_SECRET` | `--github-webhook-secret` | GitHub webhook secret (required) | - |
| `SHEPHERD_LOG_LEVEL` | `--log-level` | Log level (debug/info/warn/error) | `info` |
| `SHEPHERD_LOG_JSON` | `--log-json` | Output logs in JSON format | `false` |

### Endpoints

- `GET /health` - Health check endpoint
- `POST /hooks/github/app` - GitHub webhook receiver

## GitHub App Setup

1. Create a GitHub App in your repository or organization settings
2. Configure webhook URL: `https://your-domain.com/hooks/github/app`
3. Generate a webhook secret and set it as `SHEPHERD_GITHUB_WEBHOOK_SECRET`
4. Subscribe to events:
   - Pull requests (opened)
   - Releases (released)
   - Push

## Development

### Prerequisites

- Go 1.21 or later

### Build

```bash
go build -o shepherd
```

### Run tests

```bash
go test ./...
```

### Code quality

```bash
# Format code
go fmt ./...

# Vet code
go vet ./...

# Tidy dependencies
go mod tidy
```

## Architecture

Shepherd follows Clean Architecture principles:

```
┌─────────────────────────────────────────┐
│           Controller Layer              │
│  (HTTP handlers, routing, middleware)   │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│            UseCase Layer                │
│      (Business logic processing)        │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│            Domain Layer                 │
│  (Models, interfaces, types)            │
└─────────────────────────────────────────┘
```

## License

Apache License 2.0
